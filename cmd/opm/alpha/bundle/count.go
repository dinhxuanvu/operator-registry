package bundle

import (
	"encoding/csv"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	v1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

var (
	dir string
)

// newBundleBuildCmd returns a command that will build operator bundle image.
func newBundleCountCmd() *cobra.Command {
	bundleCountCmd := &cobra.Command{
		Use:   "count",
		Short: "count",
		Long:  "count",
		RunE:  countFunc,
	}

	bundleCountCmd.Flags().StringVarP(&dir, "directory", "d", "",
		"The directory where bundle manifests and metadata for a specific version are located")
	if err := bundleCountCmd.MarkFlagRequired("directory"); err != nil {
		log.Fatalf("Failed to mark `directory` flag for `build` subcommand as required")
	}

	return bundleCountCmd
}

func countFunc(cmd *cobra.Command, args []string) error {
	Count(dir)
	return nil
}

func Count(directory string) {
	var outputs [][]string
	// Read the root directory
	items, _ := ioutil.ReadDir(directory)
	for _, item := range items {
		// Only care about directory
		if item.IsDir() {
			var counter int
			var versions []string
			var output []string
			format := "flat"
			name := item.Name()
			// Read all files in directory
			items2, _ := ioutil.ReadDir(filepath.Join(directory, item.Name()))
			for _, item2 := range items2 {
				if item2.IsDir() {
					format = "nested"
					c, v := parseDir(filepath.Join(directory, item.Name(), item2.Name()))
					counter += c
					versions = append(versions, v...)
				}
				fileWithPath := filepath.Join(directory, item.Name(), item2.Name())
				fileBlob, _ := ioutil.ReadFile(fileWithPath)

				dec := k8syaml.NewYAMLOrJSONDecoder(strings.NewReader(string(fileBlob)), 10)
				unst := &unstructured.Unstructured{}
				if err := dec.Decode(unst); err == nil {
					if unst.GetObjectKind().GroupVersionKind().Kind == "ClusterServiceVersion" {
						// increment counter
						counter++
						csv := &v1.ClusterServiceVersion{}
						runtime.DefaultUnstructuredConverter.FromUnstructured(unst.Object, csv)
						versions = append(versions, csv.Spec.Version.String())
					}
				}
			}
			output = append(output, name, strconv.Itoa(counter), strings.Join(versions, " "), format)
			outputs = append(outputs, output)
		}
	}
	w := csv.NewWriter(os.Stdout)
	w.WriteAll(outputs)
}

func parseDir(dir string) (int, []string) {
	var counter int
	var versions []string
	items2, _ := ioutil.ReadDir(dir)
	for _, item2 := range items2 {
		if item2.IsDir() {
			c, v := parseDir(filepath.Join(dir, item2.Name()))
			counter += c
			versions = append(versions, v...)
		}
		fileWithPath := filepath.Join(dir, item2.Name())
		fileBlob, _ := ioutil.ReadFile(fileWithPath)

		dec := k8syaml.NewYAMLOrJSONDecoder(strings.NewReader(string(fileBlob)), 10)
		unst := &unstructured.Unstructured{}
		if err := dec.Decode(unst); err == nil {
			if unst.GetObjectKind().GroupVersionKind().Kind == "ClusterServiceVersion" {
				// increment counter
				counter++
				csv := &v1.ClusterServiceVersion{}
				runtime.DefaultUnstructuredConverter.FromUnstructured(unst.Object, csv)
				versions = append(versions, csv.Spec.Version.String())
			}
		}
	}
	return counter, versions
}
