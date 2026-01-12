package html

import (
	"log/slog"
	"sort"
	"strings"

	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/flightdeck/internal"
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
	"sigs.k8s.io/yaml"
)

func SiteConfigTable(site positcov1beta1.Site) Node {
	return Div(
		H2(Text("Product Image Overview"), Class("text-3xl font-bold text-gray-800 dark:text-white")),
		Class("container mx-auto py-8"),
		Table(
			Class("table-auto w-2/3 border-collapse border border-gray-300 dark:border-gray-700"),
			TBody(
				If(!internal.IsEmptyStruct(site.Spec.Workbench),
					SiteConfigTableRow("Workbench Image", site.Spec.Workbench.Image)),
				If(!internal.IsEmptyStruct(site.Spec.Connect),
					SiteConfigTableRow("Connect Image", site.Spec.Connect.Image)),
				If(!internal.IsEmptyStruct(site.Spec.PackageManager),
					SiteConfigTableRow("Package Manager Image", site.Spec.PackageManager.Image)),
			),
		),
	)
}

func SiteConfigTableRow(key string, value string) Node {
	return Tr(
		Td(Text(key), Class("text-left text-lg font-bold text-gray-800 dark:text-white border border-gray-300 dark:border-gray-700")),
		Td(Text(value), Class("text-left text-lg text-gray-800 dark:text-white border border-gray-300 dark:border-gray-700")),
	)
}

func SiteConfigBlock(site positcov1beta1.Site) Node {
	site.SetManagedFields(nil)

	productConfigs := map[string]interface{}{}

	if !internal.IsEmptyStruct(site.Spec.Workbench) {
		productConfigs["Workbench"] = site.Spec.Workbench
	}
	if !internal.IsEmptyStruct(site.Spec.Connect) {
		productConfigs["Connect"] = site.Spec.Connect
	}
	if !internal.IsEmptyStruct(site.Spec.PackageManager) {
		productConfigs["Package Manager"] = site.Spec.PackageManager
	}
	if !internal.IsEmptyStruct(site.Spec.Chronicle) {
		productConfigs["Chronicle"] = site.Spec.Chronicle
	}

	keys := make([]string, 0, len(productConfigs))
	for k := range productConfigs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return Div(
		H2(Text("Product Configurations"), Class("text-3xl font-bold text-gray-800 dark:text-white")),
		Br(),
		Class("container mx-auto py-8"),
		Group(func() []Node {
			var nodes []Node
			for _, v := range keys {
				f, err := yaml.Marshal(productConfigs[v])
				txt := strings.Replace(string(f), "\n", "<br>", -1)
				if err != nil {
					slog.Error("error generating site details", "error", err)
					return nil
				}

				nodes = append(nodes, Div(
					H3(Text(v), Class("text-2xl font-bold text-gray-800 dark:text-white")),
					Pre(
						Code(Raw(txt), Class("text-lg font-bold text-gray-800 dark:text-white")),
						Class("bg-gray-200 dark:bg-gray-800 p-4 rounded-lg"),
					),
					Br(),
				))
			}
			return nodes
		}()),
	)
}

func SiteConfigPage(site *positcov1beta1.Site, config *internal.ServerConfig) Node {
	return page("Config", config,
		Main(
			SiteConfigTable(*site),
			SiteConfigBlock(*site),
		),
	)
}
