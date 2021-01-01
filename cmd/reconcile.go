/*
Copyright © 2020 Sam Chinellato <samuele.chinellato@bt.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// reconcileCmd represents the reconcile command
var reconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Reconcile a yaml/json file/directory against your current cluster context",
	Long: `Reconcile a yaml/json file/directory against your current cluster context.
	
Target resources using the -f flag:

	k8s-sync reconcile -f <path-to-file-or-dir>
	
If a directory is selected, k8s-sync will try to apply all resources within it. It will also parse down any subdirectories.

To reapply resources every 10 seconds against your curent cluster context:

	k8s-sync reconcile -f <path-to-file-or-dir> -i 10`,
	Run: func(cmd *cobra.Command, args []string) {

		interval, err := cmd.Flags().GetInt("interval")
		if err != nil {
			log.Fatal(err)
		}

		filename, err := cmd.Flags().GetString("file")
		if err != nil {
			log.Fatal(err)
		}

		fileInfo := FileCheck(filename)

		// Setup START
		home := homedir.HomeDir()
		kubeconfig := filepath.Join(home, ".kube", "config")
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)

		if err != nil {
			panic(err.Error())
		}
		clientSet, err := kubernetes.NewForConfig(config)
		if err != nil {
			panic(err.Error())
		}

		dynamicSet, err := dynamic.NewForConfig(config)
		if err != nil {
			panic(err.Error())
		}

		if fileInfo.IsDir() {
			// Create an array for all file->bytes
			dirBytesArray, err := DirToBytes(filename)
			if err != nil {
				log.Fatal(err)
			}
			gvkUnstructuredObjMap := make(map[*schema.GroupVersionKind]*unstructured.Unstructured)
			groupResource, err := restmapper.GetAPIGroupResources(clientSet.Discovery())
			if err != nil {
				log.Fatal(err)
			}
			// For each fileBytes array, generate a map of gvk and JSON object. Append it to gvkUnstructuredObjMap
			for _, fileBytes := range dirBytesArray {
				decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(fileBytes), 100)
				tempMap := FileBytesToUnstructuredObjGVKMap(decoder, fileBytes)
				for key, value := range tempMap {
					gvkUnstructuredObjMap[key] = value
				}
			}
			mapper := restmapper.NewDiscoveryRESTMapper(groupResource)
			SyncGVKUnstructuredObjMapToK8s(gvkUnstructuredObjMap, mapper, dynamicSet, interval)
		} else {
			// file logic
			fileBytes, err := FileToBytes(filename)
			if err != nil {
				fmt.Printf("Unable to read %s", filename)
				log.Fatal(err)
			}
			decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(fileBytes), 100)
			groupResource, err := restmapper.GetAPIGroupResources(clientSet.Discovery())
			if err != nil {
				log.Fatal(err)
			}
			gvkUnstructuredObjMap := FileBytesToUnstructuredObjGVKMap(decoder, fileBytes)
			mapper := restmapper.NewDiscoveryRESTMapper(groupResource)
			SyncGVKUnstructuredObjMapToK8s(gvkUnstructuredObjMap, mapper, dynamicSet, interval)
		}
		if err != nil {
			log.Fatal("eof ", err)
		}
		fmt.Println("reconciled all resources against the cluster!")
	},
}

// FileCheck returns a os.FileInfo object if file is found
func FileCheck(filename string) os.FileInfo {
	fileInfo, err := os.Stat(filename)
	if os.IsNotExist(err) {
		log.Fatal(err)
	}
	return fileInfo
}

// FileToBytes takes in a string and returns a byte array
func FileToBytes(filename string) ([]byte, error) {
	fileBytes, err := ioutil.ReadFile(filename)
	return fileBytes, err
}

// AddTrailingSlash takes a string and adds a trailing slash if not found
func AddTrailingSlash(dirname string) string {
	match, err := regexp.MatchString("\\/$", dirname)
	if err != nil {
		log.Fatal(err)
	}
	if match != true {
		dirname = dirname + "/"
	}
	return dirname
}

// DirToBytes takes a directory as string ang returns an array of byt Arrays
func DirToBytes(dirname string) ([][]byte, error) {
	files, err := ioutil.ReadDir(dirname)
	var dirBytes [][]byte
	for _, file := range files {
		checkedDir := AddTrailingSlash(dirname)
		filename := checkedDir + file.Name()
		fileInfo := FileCheck(filename)
		// if file is a directory, parse down it recursively
		if fileInfo.IsDir() {
			fmt.Printf("%s is a directory, reading... \n", filename)
			subDirBytes, err := DirToBytes(filename)
			if err != nil {
				log.Fatal(err)
			}
			for _, fileBytes := range subDirBytes {
				dirBytes = append(dirBytes, fileBytes)
			}
		} else {
			fileBytes, err := FileToBytes(filename)
			if err != nil {
				fmt.Printf("ignoring %s : %e \n", file.Name(), err)
			} else if filepath.Ext(file.Name()) != (".yaml") && filepath.Ext(file.Name()) != (".yml") && filepath.Ext(file.Name()) != (".json") {
				fmt.Printf("%s is not a YAML or JSON file, ignoring... \n", file.Name())
			} else {
				dirBytes = append(dirBytes, fileBytes)
			}
		}
	}
	return dirBytes, err
}

// FileBytesToUnstructuredObjGVKMap takes a yaml Decoder and file bytes and returns a map of gvk[unstructuredObj]
func FileBytesToUnstructuredObjGVKMap(decoder *yamlutil.YAMLOrJSONDecoder, fileByte []byte) map[*schema.GroupVersionKind]*unstructured.Unstructured {
	var unstructuredObjGVKMap = make(map[*schema.GroupVersionKind]*unstructured.Unstructured)
	for {
		var rawObj runtime.RawExtension
		if err := decoder.Decode(&rawObj); err != nil {
			break
		}

		obj, groupVersionKind, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		if err != nil {
			fmt.Printf("unable to decode YAML from %s! Ignoring... \n", fileByte)
		} else {
			unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
			if err != nil {
				log.Fatal(err)
			}

			unstructuredObj := &unstructured.Unstructured{Object: unstructuredMap}
			unstructuredObjGVKMap[groupVersionKind] = unstructuredObj

		}
	}
	return unstructuredObjGVKMap
}

// K8sApply applies a given resource interface and unstructured object to k8s. retryObj = true if some dependencies are not found.
func K8sApply(dynamicResourceInterface dynamic.ResourceInterface, unstructuredObj *unstructured.Unstructured) bool {
	retryObj := false
	_, err := dynamicResourceInterface.Create(context.Background(), unstructuredObj, metav1.CreateOptions{})
	switch {
	case err == nil:
		// is there is no error, assume resource creation was successful
		fmt.Printf("%s/%s created \n", unstructuredObj.GetKind(), unstructuredObj.GetName())
	case errors.IsAlreadyExists(err):
		// if the object already exists print out info
		fmt.Printf("%s/%s already exists \n", unstructuredObj.GetKind(), unstructuredObj.GetName())
	case errors.IsNotFound(err):
		// if an object is not found, it may be a missing dependency. Retry here
		log.Println(unstructuredObj.GetKind(), "/", unstructuredObj.GetName(), err)
		retryObj = true
	default:
		// if there is another error, log it to the user
		log.Println("failed to create", unstructuredObj.GetKind(), "/", unstructuredObj.GetName(), err)
	}
	return retryObj
}

// ApplyGVKUnstructuredObjMap  iterates a map of gvk[unstructuredObj] and applies the object to k8s
func ApplyGVKUnstructuredObjMap(mapper meta.RESTMapper,
	dynamicSet dynamic.Interface,
	groupVersionKind *schema.GroupVersionKind,
	unstructuredObj *unstructured.Unstructured) bool {
	mapping, err := mapper.RESTMapping(groupVersionKind.GroupKind(), groupVersionKind.Version)
	if err != nil {
		log.Fatal(err)
	}
	var dynamicResourceInterface dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		if unstructuredObj.GetNamespace() == "" {
			unstructuredObj.SetNamespace("default")
		}
		dynamicResourceInterface = dynamicSet.Resource(mapping.Resource).Namespace(unstructuredObj.GetNamespace())
	} else {
		dynamicResourceInterface = dynamicSet.Resource(mapping.Resource)
	}
	retryObj := K8sApply(dynamicResourceInterface, unstructuredObj)

	return retryObj
}

// SyncGVKUnstructuredObjMapToK8s reapplies objects that failed due to not found dependencies
func SyncGVKUnstructuredObjMapToK8s(
	gvkUnstructuredObjMap map[*schema.GroupVersionKind]*unstructured.Unstructured,
	mapper meta.RESTMapper,
	dynamicSet dynamic.Interface,
	interval int) {
	allResourceCount := len(gvkUnstructuredObjMap)
	for len(gvkUnstructuredObjMap) >= 1 {
		for groupVersionKind, unstructuredObj := range gvkUnstructuredObjMap {
			retryObj := ApplyGVKUnstructuredObjMap(mapper, dynamicSet, groupVersionKind, unstructuredObj)
			if retryObj == false {
				delete(gvkUnstructuredObjMap, groupVersionKind)
			} else {
				createdResourceCount := allResourceCount - len(gvkUnstructuredObjMap)
				fmt.Printf("%d of %d  resources created... \n", createdResourceCount, allResourceCount)
				fmt.Printf("could not reconcile all objects against the cluster, retrying in %d seconds...\n", interval)
				applyInterval := rand.Int31n(int32(interval))
				time.Sleep(time.Duration(applyInterval) * time.Second)
			}
		}

	}
}

func init() {
	rootCmd.AddCommand(reconcileCmd)
	reconcileCmd.Flags().StringP("file", "f", "", "A YAML or JSON file to pass to the reconciler")
	reconcileCmd.Flags().IntP("interval", "i", 5, "Set the interval at which resources should be created")
}
