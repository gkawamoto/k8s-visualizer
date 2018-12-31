package dependency

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/gkawamoto/kube-second-mate/k8s"
	yaml "gopkg.in/yaml.v2"
)

// Graph ?
type Graph struct {
	entities      []*Entity
	hash          map[string]*Entity
	referenceHash map[string]string
}

// BuildGraph ?
func BuildGraph(target string) (*Graph, error) {
	var result = Graph{
		hash:          map[string]*Entity{},
		referenceHash: map[string]string{},
	}
	var err error
	result.entities, err = retrieveEntities(target)
	if err != nil {
		return nil, err
	}
	err = result.buildEdges()
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Entities ?
func (g *Graph) Entities() []*Entity {
	return g.entities
}

// References ?
func (g *Graph) References() map[int]int {
	var result = map[int]int{}
	var from, to string
	for from, to = range g.referenceHash {
		result[g.hash[from].ID] = g.hash[to].ID
	}
	return result
}

func (g *Graph) buildEdges() error {
	var err error
	var index int
	var entity *Entity
	for index, entity = range g.entities {
		entity.ID = index
		entity.uid = entityUID(entity)
		g.hash[entity.uid] = entity
	}
	for index, entity = range g.entities {
		err = g.resolveDependencies(entity)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *Graph) resolveDependencies(entity *Entity) error {
	var err error
	switch entity.Kind {
	case "Ingress":
		err = g.resolveIngressDependencies(entity)
	case "Service":
		err = g.resolveServiceDependencies(entity)
	case "Deployment":
		err = g.resolveDeploymentDependencies(entity)
	case "DaemonSet":
		err = g.resolveDaemonSetDependencies(entity)
	}
	return err
}

func (g *Graph) resolveIngressDependencies(entity *Entity) error {
	var obj k8s.Ingress
	var err = yaml.Unmarshal([]byte(entity.raw), &obj)
	if err != nil {
		return err
	}
	var rule k8s.IngressRule
	for _, rule = range obj.Spec.Rules {
		var httpPath k8s.IngressRuleHTTPPath
		for _, httpPath = range rule.HTTP.Paths {
			var e *Entity
			var ok bool
			var uid = kindNameUID("Service", httpPath.Backend.ServiceName)
			e, ok = g.hash[uid]
			if !ok {
				e.uid = uid
				e.ID = len(g.entities)
				e.Kind = "UnknownService"
				e.Metadata.Name = httpPath.Backend.ServiceName
				g.entities = append(g.entities, e)
				g.hash[uid] = e
			}
			g.referenceHash[entity.uid] = uid
		}
	}
	return nil
}

func (g *Graph) resolveServiceDependencies(entity *Entity) error {
	var obj k8s.Service
	var err = yaml.Unmarshal([]byte(entity.raw), &obj)
	if err != nil {
		return err
	}
	var key, value string
	var e *Entity
	for _, e = range g.entities {
		if e.Kind != "DaemonSet" && e.Kind != "Deployment" {
			continue
		}
		if e.Kind == "DaemonSet" {
			var ds k8s.DaemonSet
			err = yaml.Unmarshal([]byte(e.raw), &ds)
			if err != nil {
				return err
			}
			var found = true
			for key, value = range obj.Spec.Selector {
				var value2 string
				var ok bool
				value2, ok = ds.Metadata.Labels[key]
				if !ok {
					found = false
					break
				}
				if value != value2 {
					found = false
					break
				}
			}
			if found {
				g.referenceHash[entity.uid] = e.uid
			}
		}
		if e.Kind == "Deployment" {
			var ds k8s.Deployment
			err = yaml.Unmarshal([]byte(e.raw), &ds)
			if err != nil {
				return err
			}
			var found = true
			for key, value = range obj.Spec.Selector {
				var value2 string
				var ok bool
				value2, ok = ds.Metadata.Labels[key]
				if !ok {
					found = false
					break
				}
				if value != value2 {
					found = false
					break
				}
			}
			if found {
				g.referenceHash[entity.uid] = e.uid
			}
		}
	}
	return nil
}

func (g *Graph) resolveDaemonSetDependencies(entity *Entity) error {
	var obj k8s.DaemonSet
	var err = yaml.Unmarshal([]byte(entity.raw), &obj)
	if err != nil {
		return err
	}
	var stringServices string
	var ok bool
	stringServices, ok = obj.Metadata.Annotations["kube.references.services"]
	if !ok {
		return nil
	}
	return g.resolveKubeReferencesAnnotationsDependencies(stringServices, entity)
}
func (g *Graph) resolveKubeReferencesAnnotationsDependencies(services string, entity *Entity) error {
	var service string
	for _, service = range strings.Split(services, ",") {
		var e *Entity
		var ok bool
		var uid = kindNameUID("Service", service)
		e, ok = g.hash[uid]
		if !ok {
			e.uid = uid
			e.ID = len(g.entities)
			e.Kind = "UnknownService"
			e.Metadata.Name = service
			g.entities = append(g.entities, e)
			g.hash[uid] = e
		}
		g.referenceHash[entity.uid] = uid
	}
	return nil
}

func (g *Graph) resolveDeploymentDependencies(entity *Entity) error {
	var obj k8s.Deployment
	var err = yaml.Unmarshal([]byte(entity.raw), &obj)
	if err != nil {
		return err
	}
	var stringServices string
	var ok bool
	stringServices, ok = obj.Metadata.Annotations["kube.references.services"]
	if !ok {
		return nil
	}
	return g.resolveKubeReferencesAnnotationsDependencies(stringServices, entity)
}

func entityUID(entity *Entity) string {
	return kindNameUID(entity.Kind, entity.Metadata.Name)
}

func kindNameUID(kind, name string) string {
	return fmt.Sprintf("%s/%s", kind, name)
}

// Entity ?
type Entity struct {
	ID       int
	uid      string
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	raw string
}

// EntityReference ?
type EntityReference struct {
	from       int
	to         int
	stringFrom string
	stringTo   string
}

func retrieveEntities(target string) ([]*Entity, error) {
	var result []*Entity
	var err = filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".yaml") && !strings.HasSuffix(info.Name(), ".yml") {
			return nil
		}
		var data []byte
		data, err = ioutil.ReadFile(path)
		if err != nil {
			return fmt.Errorf("readfile: %s", err)
		}
		var entities []*Entity
		entities, err = resolveEntities(data)
		if err != nil {
			return fmt.Errorf("resolveEntities: %s", err)
		}
		result = append(result, entities...)
		return nil
	})
	return result, err
}

func resolveEntities(content []byte) ([]*Entity, error) {
	var data map[string]interface{}
	var err = yaml.Unmarshal(content, &data)
	if err != nil {
		return nil, err
	}
	var ok bool
	var kind string
	kind, ok = data["kind"].(string)
	var result = []*Entity{}
	if !ok {
		return result, nil
	}
	if kind == "List" {
		var obj interface{}
		var items []interface{}
		items, ok = data["items"].([]interface{})
		if !ok {
			return result, nil
		}
		for _, obj = range items {
			content, err = yaml.Marshal(obj)
			if err != nil {
				return nil, err
			}
			var subentities = []*Entity{}
			subentities, err = resolveEntities(content)
			if err != nil {
				return nil, err
			}
			result = append(result, subentities...)
		}
	} else {
		var e = &Entity{
			raw: string(content),
		}
		err = yaml.Unmarshal(content, e)
		if err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, nil
}
