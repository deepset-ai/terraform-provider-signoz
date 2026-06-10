package model

import (
	"encoding/json"
	"strings"

	"github.com/SigNoz/terraform-provider-signoz/signoz/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	tfattr "github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-sdk/helper/structure"
)

// Dashboard model.
type Dashboard struct {
	CollapsableRowsMigrated bool                     `json:"collapsableRowsMigrated"`
	Description             string                   `json:"description"`
	Layout                  []map[string]interface{} `json:"layout"`
	Name                    string                   `json:"name"`
	PanelMap                map[string]interface{}   `json:"panelMap"`
	Source                  string                   `json:"source"`
	Tags                    []string                 `json:"tags"`
	Title                   string                   `json:"title"`
	UploadedGrafana         bool                     `json:"uploadedGrafana"`
	Variables               map[string]interface{}   `json:"variables"`
	Version                 string                   `json:"version,omitempty"`
	Widgets                 []map[string]interface{} `json:"widgets"`
}

// The four JSON-valued attributes (layout/widgets/variables/panel_map) use
// jsontypes.Normalized so Terraform compares them by JSON semantics (key order,
// whitespace, HTML escaping ignored) at both plan and apply time. Empty maps
// normalize to "{}" (and empty layout/widgets to "[]") so they round-trip
// idempotently against SigNoz, which echoes them back unchanged.

func (d Dashboard) PanelMapToTerraform() (jsontypes.Normalized, error) {
	if d.PanelMap == nil {
		return jsontypes.NewNormalizedValue("{}"), nil
	}
	panelMap, err := structure.FlattenJsonToString(d.PanelMap)
	if err != nil {
		return jsontypes.NewNormalizedNull(), err
	}
	if panelMap == "" {
		panelMap = "{}"
	}
	return jsontypes.NewNormalizedValue(panelMap), nil
}

func (d Dashboard) VariablesToTerraform() (jsontypes.Normalized, error) {
	if len(d.Variables) == 0 {
		return jsontypes.NewNormalizedValue("{}"), nil
	}
	variables, err := structure.FlattenJsonToString(d.Variables)
	if err != nil {
		return jsontypes.NewNormalizedNull(), err
	}
	if variables == "" {
		variables = "{}"
	}
	return jsontypes.NewNormalizedValue(variables), nil
}

func (d Dashboard) TagsToTerraform() (types.List, diag.Diagnostics) {
	tags := utils.Map(d.Tags, func(value string) tfattr.Value {
		return types.StringValue(value)
	})

	return types.ListValue(types.StringType, tags)
}

func (d Dashboard) LayoutToTerraform() (jsontypes.Normalized, error) {
	if len(d.Layout) == 0 {
		return jsontypes.NewNormalizedValue("[]"), nil
	}
	b, err := json.Marshal(d.Layout)
	if err != nil {
		return jsontypes.NewNormalizedNull(), err
	}
	return jsontypes.NewNormalizedValue(string(b)), nil
}

func (d Dashboard) WidgetsToTerraform() (jsontypes.Normalized, error) {
	if len(d.Widgets) == 0 {
		return jsontypes.NewNormalizedValue("[]"), nil
	}
	b, err := json.Marshal(d.Widgets)
	if err != nil {
		return jsontypes.NewNormalizedNull(), err
	}
	return jsontypes.NewNormalizedValue(string(b)), nil
}

func (d *Dashboard) SetVariables(tfVariables jsontypes.Normalized) error {
	if tfVariables.IsNull() || tfVariables.IsUnknown() {
		d.Variables = map[string]interface{}{}
		return nil
	}
	s := tfVariables.ValueString()
	if s == "" {
		d.Variables = map[string]interface{}{}
		return nil
	}
	variables, err := structure.ExpandJsonFromString(s)
	if err != nil {
		return err
	}
	d.Variables = variables
	return nil
}

func (d *Dashboard) SetPanelMap(tfPanelMap jsontypes.Normalized) error {
	if tfPanelMap.IsNull() || tfPanelMap.IsUnknown() || tfPanelMap.ValueString() == "" {
		d.PanelMap = make(map[string]interface{})
		return nil
	}
	panelMap, err := structure.ExpandJsonFromString(tfPanelMap.ValueString())
	if err != nil {
		return err
	}
	d.PanelMap = panelMap
	return nil
}

func (d *Dashboard) SetTags(tfTags types.List) {
	tags := utils.Map(tfTags.Elements(), func(value tfattr.Value) string {
		return strings.Trim(value.String(), "\"")
	})
	d.Tags = tags
}

func (d *Dashboard) SetLayout(tfLayout jsontypes.Normalized) error {
	var layout []map[string]interface{}
	if err := json.Unmarshal([]byte(tfLayout.ValueString()), &layout); err != nil {
		return err
	}
	d.Layout = layout
	return nil
}

func (d *Dashboard) SetWidgets(tfWidgets jsontypes.Normalized) error {
	var widgets []map[string]interface{}
	if err := json.Unmarshal([]byte(tfWidgets.ValueString()), &widgets); err != nil {
		return err
	}
	d.Widgets = widgets
	return nil
}

func (d *Dashboard) SetSourceIfEmpty(hostURL string) {
	d.Source = utils.WithDefault(d.Source, hostURL+"/dashboard")
}
