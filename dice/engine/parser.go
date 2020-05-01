package engine

import (
	"sigs.k8s.io/yaml"
)
//valid "github.com/asaskevich/govalidator"

type Data []byte

// Enumeration for category of metadata in Tile specification.
type Category int
const (
	Network Category = iota
	Compute
	ContainerProvider
	Storage
	Database
	Application
	ContainerApplication
	Analysis
	ML
)
func (c Category) CString() string {
	return [...]string{"Network", "Compute", "ContainerProvider", "Storage", "Database", "Application", "ContainerApplication", "Analysis", "ML"}[c]
}

// Enumeration for input/output in Tile specification
type IOType int
const (
	String IOType = iota
	Number
	CDKObject
	FromCommand
)
func (iot IOType) IOTString() string {
	return [...]string{"String", "Number", "CDKObject", "FromCommand"}[iot]
}

// Manifest type
type ManifestType int
const (
	K8s ManifestType = iota
	Helm
	Kustomize
)
func (mts ManifestType) MTString() string {
	return [...]string{"K8s", "Helm", "Kustomize"}[mts]
}



// Deployment specification
type Deployment struct {
	ApiVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind" valid:"in(Deployment)"`
	Metadata   Metadata       `json:"metadata"`
	Spec       DeploymentSpec `json:"spec"`
}

type DeploymentSpec struct {
	Template DeploymentTemplate `json:"template"`
}

type DeploymentTemplate struct {
	Category string `json:"category" valid:"in(Network|Compute|ContainerProvider|Storage|Database|Application|ContainerApplication|Analysis|ML)"`
	Tiles []DeploymentTemplateDetail `json:"tiles"`
}


type DeploymentTemplateDetail struct {
	TileReference string      `json:"tileReference"`
	TileVersion   string      `json:"tileVersion"`
	Inputs        []TileInput `json:"inputs"`
	Manifests	TileManifest	`json:"manifests"`
}

// Tile specification
type Tile struct {
	ApiVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Metadata   Metadata `json:"metadata"`
	Spec       TileSpec `json:"spec"`
}

type Metadata struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	VendorService string `json:"vendorService"`
	Version  string `json:"version"`
}

type TileSpec struct {
	Dependencies []TileDependency `json:"dependencies"`
	Inputs       []TileInput      `json:"inputs"`
	Manifests    TileManifest     `json:"manifests"`
	Outputs      []TileOutput     `json:"outputs"`
	Notes        []string         `json:"notes"`
}

type TileDependency struct {
	Name          string `json:"name"`
	TileReference string `json:"tileReference"`
	TileVersion   string `json:"tileVersion"`
}

type TileInput struct {
	Name          string                `json:"name"`
	InputType     string                `json:"inputType"`
	Description   string                `json:"description"`
	Dependencies  []TileInputDependency `json:"dependencies"`
	DefaultValue  string                `json:"defaultValue"`
	DefaultValues []string              `json:"defaultValues"`
	InputValue    string                `json:"inputValue"`
	InputValues   []string                `json:"inputValues"`
	Require       bool                `json:"require"` // true/false
	Override TileInputOverride `json:"override"`
}

type TileInputOverride struct {
	Name  string `json:"name"`
	Field string `json:"field"`
	InputName string
	OverrideValue string
}
type TileManifest struct {
	ManifestType string   `json:"manifestType"`
	Namespace string `json:"namespace"`
	Files        []string `json:"files"`
	Folders      []string `json:"folders"`
}

type TileInputDependency struct {
	Name  string `json:"name"`
	Field string `json:"field"`
}

type TileOutput struct {
	Name                string `json:"name"`
	OutputType          string `json:"outputType"`
	DefaultValue        string `json:"defaultValue"`
	DefaultValueCommand string `json:"defaultValueCommand"`
	Description         string `json:"description"`
}

// Parsing functions
type ParserCore interface {
	ParseTile() (*Tile, error)
	ParseDeployment() (*Deployment, error)
}


// ParseTile parse Tile
func (d *Data) ParseTile() (*Tile, error) {
	var tile Tile

	err := yaml.Unmarshal(*d, &tile)
	if err != nil {
		return &tile, err
	}

	return &tile, nil
}

// ParseDeployment parse Deployment
func (d *Data) ParseDeployment() (*Deployment, error) {
	var deployment Deployment

	if err := yaml.Unmarshal(*d, &deployment); err != nil {
		return &deployment, err
	}

	return &deployment, nil
}
