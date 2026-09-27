package launcher

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
)

type launchConfig struct {
	Exe          string
	Params       string
	AccountParam string
	CodeParam    string
}

var miscNames = []string{
	"MyComCodeParam", "MyComGameAccountParam", "MyComUserIdParam",
	"SezamCodeParam", "SezamGameAccountParam", "SezamUserIdParam",
	"EXTDIAG2", "CrashPaths", "CrashFilters", "UpdateScript", "InstallScript",
	"ExeFileName", "ExeFileName64bit", "ExeParams", "ExeParams64bit", "GAMEID",
}

func attrValue(attrs []xml.Attr, name string) string {
	for _, a := range attrs {
		if strings.EqualFold(a.Name.Local, name) {
			return a.Value
		}
	}
	return ""
}

func mergeMisc(old, manifest xmlElement) xmlElement {
	canonical := func(name string) string {
		for _, c := range miscNames {
			if strings.EqualFold(c, name) {
				return c
			}
		}
		return name
	}
	merged := xmlElement{XMLName: xml.Name{Local: "Misc"}}
	if id := attrValue(old.Attrs, "GAMEID"); id != "" {
		merged.Attrs = setAttrFold(merged.Attrs, "GAMEID", id)
	}
	for _, a := range manifest.Attrs {
		merged.Attrs = setAttrFold(merged.Attrs, canonical(a.Name.Local), a.Value)
	}
	return merged
}

func setAttrFold(attrs []xml.Attr, name, value string) []xml.Attr {
	for i := range attrs {
		if strings.EqualFold(attrs[i].Name.Local, name) {
			attrs[i] = xml.Attr{Name: xml.Name{Local: name}, Value: value}
			return attrs
		}
	}
	return append(attrs, xml.Attr{Name: xml.Name{Local: name}, Value: value})
}

func readLaunchConfig(last []byte) (launchConfig, error) {
	var root struct {
		Misc xmlElement `xml:"Misc"`
	}
	if err := readXML(last, &root); err != nil {
		return launchConfig{}, fmt.Errorf("last.xml: %w", err)
	}
	misc := func(name string) string { return attrValue(root.Misc.Attrs, name) }
	pick := func(names ...string) string {
		for _, n := range names {
			if v := misc(n); v != "" {
				return v
			}
		}
		return ""
	}
	cfg := launchConfig{
		Exe:          misc("ExeFileName64bit"),
		Params:       misc("ExeParams64bit"),
		AccountParam: pick("SezamGameAccountParam", "MyComGameAccountParam"),
		CodeParam:    pick("SezamCodeParam", "MyComCodeParam"),
	}
	if cfg.Exe == "" || cfg.AccountParam == "" || cfg.CodeParam == "" {
		return launchConfig{}, errors.New("last.xml has no ExeFileName64bit or auth parameters")
	}
	return cfg, nil
}

func appendParam(args []string, param, value string) []string {
	if strings.HasSuffix(param, " ") {
		return append(args, strings.TrimSpace(param), value)
	}
	return append(args, param+value)
}

func (c launchConfig) args(ticket gameTicket) []string {
	args := strings.Fields(c.Params)
	args = appendParam(args, c.AccountParam, ticket.GameAccount)
	return appendParam(args, c.CodeParam, ticket.Code)
}
