package engine

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
)


type TilesGrid struct {
	TileInstance string
	ExecutableOrder int
	TileName string
	TileVersion string
	TileCategory string
	rootTileInstance string
	ParentTileInstance string
}


// Ts is key struct to fulfil super.ts template and key element to generate execution plan.
type Ts struct {
	// TsLibs
	TsLibs []TsLib
	// TsLibsMap : TileName -> TsLib
	TsLibsMap map[string]TsLib

	// TsStacks
	TsStacks []*TsStack
	// TsStacksMap : TileInstance -> TsStack
	// 		all initialized values will be store here, include input, env, etc
	TsStacksMapN map[string]*TsStack

	// AllTiles : TileInstance -> Tile
	AllTilesN map[string]*Tile
	// AllOutputs :  TileInstance ->TsOutput
	// 		all output values will be store here
	AllOutputsN map[string]*TsOutput

	// Created time
	CreatedTime time.Time
}

type TsLib struct {
	TileInstance string
	TileName          string
	TileVersion       string
	TileConstructName string
	TileFolder        string
	TileCategory      string
}

type TsStack struct {
	TileInstance string
	TileName          string
	TileVersion       string
	TileConstructName string
	TileVariable      string
	TileStackName     string
	TileStackVariable string
	TileCategory      string
	InputParameters   map[string]TsInputParameter //input name -> TsInputParameter
	TsManifests       *TsManifests
}

type TsInputParameter struct {
	InputName       string
	InputValue      string
	IsOverrideField string
	DependentTileInstance string
	DependentTileInputName string
}

type TsManifests struct {
	ManifestType         string
	Namespace            string
	Files                []string
	Folders              []string
	TileInstance string

}

type TsOutput struct {
	TileName    string
	TileVersion string
	StageName   string
	TsOutputs   map[string]*TsOutputDetail //OutputName -> TsOutputDetail
}

type TsOutputDetail struct {
	Name                string
	OutputType          string
	DefaultValue        string
	DefaultValueCommand string
	OutputValue         string
	Description         string
}

// AllTs represents all information about tiles, input, output, etc.,  id(uuid) -> Ts
var AllTs = make(map[string]Ts)
// AllTilesGrid store all Tiles relationship, id(uuid) -> (tile-instance -> TilesGrid)
var AllTilesGrids = make(map[string]*map[string]TilesGrid)


// SortedTilesGrid return sorted TilesGrid array from AllTilesGrid
func SortedTilesGrid(dSid string) []TilesGrid {
	if allTG, ok := AllTilesGrids[dSid]; ok {
		var tg []TilesGrid
		for _, v := range *allTG {
			tg = append(tg, v)
		}
		sort.SliceStable(tg, func(i, j int) bool {
			return tg[i].ExecutableOrder < tg[j].ExecutableOrder
		})
		return tg
	}
	return nil
}

func DependentEKSTile(dSid string, tileInstance string) *Tile {

	if allTG, ok := AllTilesGrids[dSid]; ok {
		for _, v := range *allTG {
			if v.ParentTileInstance == tileInstance {
				if at, ok := AllTs[dSid]; ok {
					if tile, ok := at.AllTilesN[v.TileInstance]; ok {
						if tile.Metadata.VendorService == EKS.VSString() {
							return tile
						}
					}
				}
			}
		}
	}

	return nil

}

func AllDependentTiles(dSid string, tileInstance string) []Tile {

	if allTG, ok := AllTilesGrids[dSid]; ok {
		var tiles []Tile
		for _, v := range *allTG {
			if v.ParentTileInstance == tileInstance {
				if at, ok := AllTs[dSid]; ok {
					if tile, ok := at.AllTilesN[v.TileInstance]; ok {
						tiles = append(tiles, *tile)
					}
				}
			}
		}
		return tiles
	}
	return nil
}

func IsDuplicatedCategory(dSid string, rootTileInstance string, tileCategory string) bool {
	if allTG, ok := AllTilesGrids[dSid]; ok {
		for _, v := range *allTG {
			if v.rootTileInstance == rootTileInstance {
				if v.TileCategory == tileCategory {
					return true
				}
			}
		}
	}
	return false
}

func ReferencedTsStack(dSid string, rootTileInstance string, tileName string) *TsStack {
	if allTG, ok := AllTilesGrids[dSid]; ok {
		for _, v := range *allTG {
			if v.rootTileInstance == rootTileInstance {
				if v.TileName == tileName {
					ts := AllTs[dSid]
					return ts.TsStacksMapN[v.TileInstance]
				}
			}
		}
	}
	return nil
}


func ValueRef(dSid string, ref string, ti string) (string, error) {
	re := regexp.MustCompile(`^\$\(([[:alnum:]]*\.[[:alnum:]]*\.[[:alnum:]]*)\)$`)
	ms := re.FindStringSubmatch(ref)
	if len(ms)==2 {
		str := strings.Split(ms[1], ".")
		tileInstance := str[0]
		where := str[1]
		field := str[2]
		if tileInstance == "self" && ti != "" {
			tileInstance = ti
		}
		if at, ok := AllTs[dSid]; ok {

				switch where {
				case "inputs":
					if tsStack, ok := at.TsStacksMapN[tileInstance]; ok {
						for _, input := range tsStack.InputParameters {
							if field == input.InputName {
								return input.InputValue, nil
							}
						}
					}
				case "outputs":
					if outputs, ok := at.AllOutputsN[tileInstance]; ok {
						for name, output := range outputs.TsOutputs {
							if name == field {
								return output.OutputValue, nil
							}
						}
					}

				}
		}

	} else {
		return "", errors.New("expression: "+ref+" was error")
	}
	return "", errors.New("referred value wasn't exist")
}

func ParentTileInstance(dSid string, tileInstance string) string {
	if allTG, ok := AllTilesGrids[dSid]; ok {
		if tg, ok := (*allTG)[tileInstance]; ok {
			return tg.ParentTileInstance
		}
	}
	return ""
}