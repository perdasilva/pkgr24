package main

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/operator-framework/operator-registry/alpha/declcfg"
	"github.com/operator-framework/operator-registry/alpha/property"
	pkgr24iov1alpha1 "github.com/perdasilva/pkgr24/api/v1alpha1"
	"go.uber.org/zap/buffer"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func main() {
	const catalogDir = "/home/perdasilva/tmp/redhat-index-4.11"
	const outputDir = "packages"

	files, err := ioutil.ReadDir(catalogDir)
	if err != nil {
		log.Fatalf("error reading catalog directory (%s): %s", catalogDir, err)
	}

	_ = os.RemoveAll(outputDir)
	_ = os.Mkdir("packages", 0774)

	catalogFs := os.DirFS(catalogDir)
	for _, file := range files {
		if !file.IsDir() {
			log.Printf("ignoring file: %s", file.Name())
			continue
		}
		log.Printf("exporting package %s", file.Name())
		catalogPath := path.Join(catalogDir, file.Name(), "catalog.json")
		cfg, err := leanCatalog(catalogFs, path.Join(file.Name(), "catalog.json"))
		if err != nil {
			log.Fatalf("error loading catalog (%s): %s", catalogPath, err)
		}
		buff := buffer.Buffer{}
		err = declcfg.WriteJSON(*cfg, &buff)
		if err != nil {
			log.Fatalf("failed to convert decl cfg to string: %s", err)
		}

		pkgObj := &unstructured.Unstructured{}
		pkgObj.Object = map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": file.Name(),
			},
			"spec": map[string]interface{}{
				"packageFBC": buff.String(),
			},
		}
		pkgObj.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   pkgr24iov1alpha1.GroupVersion.Group,
			Version: pkgr24iov1alpha1.GroupVersion.Version,
			Kind:    "Package",
		})
		out, err := yaml.Marshal(pkgObj.Object)
		if err != nil {
			log.Fatalf("failed to export catalog (%s): %s", catalogPath, err)
		}

		err = os.WriteFile(path.Join(outputDir, fmt.Sprintf("%s.yaml", file.Name())), out, 0664)
		if err != nil {
			log.Fatalf("failed to write exported catalog (%s): %s", catalogPath, err)
		}
	}
}

func leanCatalog(fs fs.FS, packageCatalogPath string) (*declcfg.DeclarativeConfig, error) {
	cfg, err := declcfg.LoadFile(fs, packageCatalogPath)
	if err != nil {
		return nil, fmt.Errorf("error loading catalog (%s): %s", packageCatalogPath, err)
	}

	// remove icon data
	for i, _ := range cfg.Packages {
		cfg.Packages[i].Icon = nil
	}

	// remove bundle binary data
	for i, _ := range cfg.Bundles {
		props := make([]property.Property, 0)
		for j, _ := range cfg.Bundles[i].Properties {
			if cfg.Bundles[i].Properties[j].Type != "olm.bundle.object" {
				props = append(props, cfg.Bundles[i].Properties[j])
			}
		}
		cfg.Bundles[i].Properties = props
	}

	return cfg, nil
}
