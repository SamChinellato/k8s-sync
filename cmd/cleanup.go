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
	"log"
	"path/filepath"
	"os"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Delete a yaml/json file/directory against your current cluster context",
	Long: `Delete a yaml/json file/directory against your current cluster context.
To delete resources in a file or directory:

	k8s-sync cleanup -f <path-to-file-or-dir>

If a directory is specified, k8s-sync will try to delete all resources in the directory and any subdirectories from the current cluster
context Kubernetes cluster.	`,
	Run: func(cmd *cobra.Command, args []string) {
		filename, err := cmd.Flags().GetString("file")
		if err != nil {
			log.Fatal(err)
		}
		if filename == "" {
			fmt.Println("Please set a file or directory with: \n    'k8s-sync cleanup -f <target>' ")
			os.Exit(2)
		}

		fileInfo := FileCheck(filename)
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
			for groupVersionKind, unstructuredObj := range gvkUnstructuredObjMap {
				K8sDelete(mapper, groupVersionKind, dynamicSet, unstructuredObj)
			}
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
			for groupVersionKind, unstructuredObj := range gvkUnstructuredObjMap {
				K8sDelete(mapper, groupVersionKind, dynamicSet, unstructuredObj)
			}
		}
		if err != nil {
			log.Fatal("eof ", err)
		}
		fmt.Println("cleaned up all resources against the cluster!")
	},
}

// K8sDelete Takes in a mapper, gvk and Ustructured obj and applies it against the cluster.
func K8sDelete(mapper meta.RESTMapper, groupVersionKind *schema.GroupVersionKind, dynamicSet dynamic.Interface, unstructuredObj *unstructured.Unstructured) {
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
	err = dynamicResourceInterface.Delete(context.Background(), unstructuredObj.GetName(), metav1.DeleteOptions{})
	if err != nil {
		log.Println("failed to delete", unstructuredObj.GetKind(), "/", unstructuredObj.GetName(), err)
	} else {
		fmt.Printf("deleted %s/%s \n", unstructuredObj.GetKind(), unstructuredObj.GetName())
	}

}
func init() {
	rootCmd.AddCommand(cleanupCmd)
	cleanupCmd.Flags().StringP("file", "f", "", "A YAML or JSON file to pass to the reconciler")
}
