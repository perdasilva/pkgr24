package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/go-air/gini"
	"github.com/go-air/gini/logic"
	"github.com/go-air/gini/z"
	"github.com/operator-framework/operator-registry/pkg/api"
	"github.com/perdasilva/pkgr24/pkg/solver"
)

const (
	gvkRequired     = "olm.gvk.required"
	packageRequired = "olm.package.required"
	// labelRequired   = "olm.label.required"
)

func main() {
	const cacheFile = "/home/perdasilva/tmp/redhat-index-cache.bin"
	bundles, err := getBundleList(cacheFile)
	if err != nil {
		return
	}
	//circuit, constraints, err := solveWithGini(bundles)
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}

	if err := solveWithSolver(bundles); err != nil {
		fmt.Println(err)
		return
	}
}

func getBundleList(cacheFile string) ([]*api.Bundle, error) {
	byts, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, err
	}
	decoder := gob.NewDecoder(bytes.NewReader(byts))
	var bundles []*api.Bundle
	err = decoder.Decode(&bundles)
	return bundles, err
}

func bundleListToVariables(bundles []*api.Bundle) []solver.Variable {
	for _, bundle := range bundles {
		if bundle.Dependencies != nil {
			fmt.Println(bundle.PackageName)
		}
	}
	return nil
}

func solveWithGini(bundles []*api.Bundle) (*logic.C, []z.Lit, error) {
	circuit := logic.NewC()

	bundleToLit := map[*api.Bundle]z.Lit{}
	pkgToBundles := map[string][]*api.Bundle{}
	gvkToBundles := map[*api.GroupVersionKind][]*api.Bundle{}
	lits := make([]z.Lit, len(bundles))
	var constraints []z.Lit

	// add a lit for each bundle
	for i, bundle := range bundles {
		bundleLit := circuit.Lit()
		bundleToLit[bundles[i]] = bundleLit
		if strings.Contains(bundle.PackageName, "odf-multicluster-orchestrator") {
			fmt.Printf("index=%d version=%s\n", i, bundle.Version)
		}
		pkgToBundles[bundle.PackageName] = append(pkgToBundles[bundle.PackageName], bundle)
		for _, gvk := range bundle.ProvidedApis {
			gvkToBundles[gvk] = append(gvkToBundles[gvk], bundle)
		}
		lits[i] = bundleLit
	}

	// at most one bundle / package
	for _, bundles := range pkgToBundles {
		bundleLits := make([]z.Lit, len(bundles))
		for i, bundle := range bundles {
			bundleLits[i] = bundleToLit[bundle]
		}
		constraints = append(constraints, circuit.CardSort(bundleLits).Leq(1))
	}

	// at most one bundle per gvk
	for _, bundles := range gvkToBundles {
		bundleLits := make([]z.Lit, len(bundles))
		for i, bundle := range bundles {
			bundleLits[i] = bundleToLit[bundle]
		}
		constraints = append(constraints, circuit.CardSort(bundleLits).Leq(1))
	}

	// add package dependencies
	for _, bundle := range bundles {
		var deps []z.Lit
		if bundle.PackageName == "odf-multicluster-orchestrator" && bundle.Version == "4.11.0" {
			fmt.Println("hello")
		}
		for _, dependency := range bundle.Dependencies {
			switch dependency.Type {
			case "olm.gvk":
				fallthrough
			case gvkRequired:
				gvk := &api.GroupVersionKind{}
				if err := json.Unmarshal([]byte(dependency.Value), gvk); err != nil {
					return nil, nil, err
				}
				for _, provider := range gvkToBundles[gvk] {
					deps = append(deps, bundleToLit[provider])
				}
			case "olm.package":
				fallthrough
			case packageRequired:
				var pkg struct {
					PackageName  string `json:"packageName"`
					VersionRange string `json:"version"`
				}
				if err := json.Unmarshal([]byte(dependency.Value), &pkg); err != nil {
					return nil, nil, err
				}
				versionRange, err := semver.ParseRange(pkg.VersionRange)
				if err != nil {
					return nil, nil, err
				}
				for _, pkgBundle := range pkgToBundles[pkg.PackageName] {
					bundleVersion, err := semver.Parse(pkgBundle.Version)
					if err != nil {
						return nil, nil, err
					}
					if versionRange(bundleVersion) {
						deps = append(deps, bundleToLit[pkgBundle])
					}
				}
			default:
				// fmt.Println("Unknown dependency: ", dependency.Type)
			}
		}
		if deps != nil {
			constraints = append(constraints, circuit.Ors(append(deps, bundleToLit[bundle].Not())...))
		}
	}

	bundleIndex := 547
	satSolver := gini.New()
	satSolver.Assume(constraints...)
	satSolver.Assume(bundleToLit[bundles[bundleIndex]])
	cs := circuit.CardSort(lits)
	circuit.ToCnf(satSolver)
	satSolver.Test(lits)
	for w := 0; w <= cs.N(); w++ {
		satSolver.Assume(cs.Leq(w))
		if satSolver.Solve() == 1 {
			break
		} else {
			fmt.Println("failed for w=" + strconv.Itoa(w))
		}
	}

	fmt.Println("* " + bundles[bundleIndex].PackageName + " " + bundles[0].Version)
	fmt.Printf("deps: %s\n", bundles[bundleIndex].Dependencies)
	// fmt.Println(satSolver.Solve())

	for bundle, lit := range bundleToLit {
		if satSolver.Value(lit) {
			fmt.Println(bundle.PackageName + " " + bundle.Version)
		}
	}

	return circuit, constraints, nil
}

func solveWithSolver(bundles []*api.Bundle) error {

	pkgToBundles := map[string][]*api.Bundle{}
	gvkToBundles := map[*api.GroupVersionKind][]*api.Bundle{}
	bundleToVariable := map[*api.Bundle]*BundleVariable{}

	// add a lit for each bundle
	for _, bundle := range bundles {
		pkgToBundles[bundle.PackageName] = append(pkgToBundles[bundle.PackageName], bundle)
		for _, gvk := range bundle.ProvidedApis {
			gvkToBundles[gvk] = append(gvkToBundles[gvk], bundle)
		}
		bundleToVariable[bundle] = NewBundleVariable(bundle)
	}

	// add package dependencies
	for _, bundle := range bundles {
		if bundle.PackageName == "web-terminal" && bundle.Version == "1.4.0" {
			bundleToVariable[bundle].AddConstraint(solver.Mandatory())
		}
		var deps []solver.Identifier
		for _, dependency := range bundle.Dependencies {
			switch dependency.Type {
			case "olm.gvk":
				fallthrough
			case gvkRequired:
				gvk := &api.GroupVersionKind{}
				if err := json.Unmarshal([]byte(dependency.Value), gvk); err != nil {
					return err
				}
				for _, provider := range gvkToBundles[gvk] {
					deps = append(deps, bundleToVariable[provider].Identifier())
				}
			case "olm.package":
				fallthrough
			case packageRequired:
				var pkg struct {
					PackageName  string `json:"packageName"`
					VersionRange string `json:"version"`
				}
				if err := json.Unmarshal([]byte(dependency.Value), &pkg); err != nil {
					return err
				}
				versionRange, err := semver.ParseRange(pkg.VersionRange)
				if err != nil {
					return err
				}
				for _, pkgBundle := range pkgToBundles[pkg.PackageName] {
					bundleVersion, err := semver.Parse(pkgBundle.Version)
					if err != nil {
						return err
					}
					if versionRange(bundleVersion) {
						deps = append(deps, bundleToVariable[pkgBundle].Identifier())
					}
				}
			default:
				// fmt.Println("Unknown dependency: ", dependency.Type)
			}
		}
		if deps != nil {
			bundleToVariable[bundle].AddConstraint(solver.Dependency(deps...))
		}
	}

	variables := make([]solver.Variable, 0, len(bundleToVariable))
	for _, bundleVariable := range bundleToVariable {
		variables = append(variables, bundleVariable)
	}

	pkgSolver, err := solver.New(solver.WithInput(variables))
	if err != nil {
		return err
	}

	solution, err := pkgSolver.Solve(context.Background())
	if err != nil {
		return err
	}

	fmt.Println("---------------------------------")
	for _, item := range solution {
		fmt.Println(item.Identifier())
	}
	fmt.Println("---------------------------------")

	return nil
}

type BundleVariable struct {
	apiBundle   *api.Bundle
	constraints []solver.Constraint
}

func NewBundleVariable(bundle *api.Bundle) *BundleVariable {
	return &BundleVariable{
		apiBundle:   bundle,
		constraints: nil,
	}
}

func (v *BundleVariable) Identifier() solver.Identifier {
	return solver.Identifier(strings.Join([]string{v.apiBundle.ChannelName, v.apiBundle.PackageName, v.apiBundle.Version}, "/"))
}

func (v *BundleVariable) Constraints() []solver.Constraint {
	return v.constraints
}

func (v *BundleVariable) AddConstraint(constraint solver.Constraint) {
	v.constraints = append(v.constraints, constraint)
}

func (v *BundleVariable) GetBundle() *api.Bundle {
	return v.apiBundle
}

func testGini() {
	circuit := logic.NewC()
	a := circuit.Lit()
	b := circuit.Lit()
	g := gini.New()

	c := circuit.Ands(a, b)
	circuit.ToCnf(g)
	g.Assume(c)
	fmt.Println(g.Solve())
	fmt.Println(g.Value(a), g.Value(b), g.Value(c))
}
