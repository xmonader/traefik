package provider

import (
	"bytes"
	"github.com/containous/traefik/pkg/config"
	"text/template"
	//"io/ioutil"
	"strings"
	//"unicode"

	"github.com/BurntSushi/toml"
	"github.com/Masterminds/sprig"
	"github.com/containous/traefik/pkg/log"
	"github.com/containous/traefik/pkg/safe"
	"github.com/containous/traefik/pkg/types"

)


// Provider defines methods of a provider.
type Provider interface {
	// Provide allows the provider to provide configurations to traefik
	// using the given configuration channel.
	Provide(configurationChan chan<- config.Message, pool *safe.Pool) error
	Init() error
}



type BaseProvider struct {
	Watch                     bool              `description:"Watch provider" export:"true"`
	Filename                  string            `description:"Override default configuration template. For advanced users :)" export:"true"`
	Constraints               types.Constraints `description:"Filter services by constraint, matching with Traefik tags." export:"true"`
	Trace                     bool              `description:"Display additional provider logs (if available)." export:"true"`
	TemplateVersion           int               `description:"Template version." export:"true"`
	DebugLogGeneratedTemplate bool              `description:"Enable debug logging of generated configuration template." export:"true"`
}

// Init for compatibility reason the BaseProvider implements an empty Init
func (p *BaseProvider) Init(constraints types.Constraints) error {
	p.Constraints = append(p.Constraints, constraints...)
	return nil
}

// MatchConstraints must match with EVERY single constraint
// returns first constraint that do not match or nil
func (p *BaseProvider) MatchConstraints(tags []string) (bool, *types.Constraint) {
	// if there is no tags and no constraints, filtering is disabled
	if len(tags) == 0 && len(p.Constraints) == 0 {
		return true, nil
	}

	for _, constraint := range p.Constraints {
		// xor: if ok and constraint.MustMatch are equal, then no tag is currently matching with the constraint
		if ok := constraint.MatchConstraintWithAtLeastOneTag(tags); ok != constraint.MustMatch {
			return false, constraint
		}
	}

	// If no constraint or every constraints matching
	return true, nil
}

// GetConfiguration return the provider configuration from default template (file or content) or overrode template file
func (p *BaseProvider) GetConfiguration(defaultTemplate string, funcMap template.FuncMap, templateObjects interface{}) (*Configuration, error) {
	tmplContent, err := p.getTemplateContent(defaultTemplate)
	if err != nil {
		return nil, err
	}
	return p.CreateConfiguration(tmplContent, funcMap, templateObjects)
}

// CreateConfiguration create a provider configuration from content using templating
func (p *BaseProvider) CreateConfiguration(tmplContent string, funcMap template.FuncMap, templateObjects interface{}) (*Configuration, error) {
	var defaultFuncMap = sprig.TxtFuncMap()
	// tolower is deprecated in favor of sprig's lower function
	defaultFuncMap["tolower"] = strings.ToLower
	defaultFuncMap["normalize"] = Normalize
	defaultFuncMap["split"] = split
	for funcID, funcElement := range funcMap {
		defaultFuncMap[funcID] = funcElement
	}

	tmpl := template.New(p.Filename).Funcs(defaultFuncMap)

	_, err := tmpl.Parse(tmplContent)
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	err = tmpl.Execute(&buffer, templateObjects)
	if err != nil {
		return nil, err
	}

	var renderedTemplate = buffer.String()
	if p.DebugLogGeneratedTemplate {
		log.Debugf("Template content: %s", tmplContent)
		log.Debugf("Rendering results: %s", renderedTemplate)
	}
	return p.DecodeConfiguration(renderedTemplate)
}

// DecodeConfiguration Decode a *Configuration from a content
func (p *BaseProvider) DecodeConfiguration(content string) (*Configuration, error) {
	configuration := new(Configuration)
	if _, err := toml.Decode(content, configuration); err != nil {
		return nil, err
	}
	return configuration, nil
}

//func (p *BaseProvider) getTemplateContent(defaultTemplateFile string) (string, error) {

//	if len(p.Filename) > 0 {
//		buf, err := ioutil.ReadFile(p.Filename)
//		if err != nil {
//			return "", err
//		}
//		return string(buf), nil
//	}
//
//	if strings.HasSuffix(defaultTemplateFile, ".tmpl") {
//		buf, err := gentemplates.Asset(defaultTemplateFile)
//		if err != nil {
//			return "", err
//		}
//		return string(buf), nil
//	}
//
//	return defaultTemplateFile, nil
//}

func split(sep, s string) []string {
	return strings.Split(s, sep)
}


// ReverseStringSlice invert the order of the given slice of string
func ReverseStringSlice(slice *[]string) {
	for i, j := 0, len(*slice)-1; i < j; i, j = i+1, j-1 {
		(*slice)[i], (*slice)[j] = (*slice)[j], (*slice)[i]
	}
}
