package html

import (
	"fmt"

	positcov1beta1 "github.com/posit-dev/team-operator/api/core/v1beta1"
	"github.com/posit-dev/team-operator/flightdeck/internal"
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

func HomePage(site positcov1beta1.Site, config *internal.ServerConfig) Node {
	baseUrl := site.Spec.Domain
	return page("Home", config,
		Main(
			Class("container mx-auto py-8 px-4"),
			Div(
				Class("text-center mb-12 mt-8 mx-auto max-w-4xl"),
				H2(
					Class("text-3xl md:text-4xl font-semibold text-gray-800 dark:text-white mb-6 leading-tight"),
					Text("Welcome to Posit Team, your end-to-end platform for creating amazing data products."),
				),
				P(
					Class("text-gray-600 text-lg md:text-xl dark:text-gray-300 leading-relaxed"),
					Text("Quickly build and share code and applications while seamlessly collaborating with your colleagues and stakeholders."),
				),
			),
			Div(
				Class("grid grid-cols-1 md:grid-cols-3 gap-6 max-w-6xl mx-auto"),
				If(!internal.IsEmptyStruct(site.Spec.Workbench),
					productCard("/static/logo-workbench.svg", "Posit Workbench", site.Spec.Workbench.DomainPrefix, baseUrl,
						"Manage your environments with integrated tools like JupyterLab, RStudio, VS Code and Positron. "+
							"Self-service workspaces provide a secure solution for both on-premises and cloud deployments"),
				),
				If(!internal.IsEmptyStruct(site.Spec.Connect),
					productCard("/static/logo-connect.svg", "Posit Connect", site.Spec.Connect.DomainPrefix, baseUrl,
						"Share your interactive applications, dashboards, and reports built with R and Python. "+
							"Manage access, and deliver real-time insights to your stakeholders."),
				),
				If(!internal.IsEmptyStruct(site.Spec.PackageManager),
					productCard("/static/logo-packagemanager.svg", "Posit Package Manager", site.Spec.PackageManager.DomainPrefix, baseUrl,
						"Securely manage your R and Python packages from public and internal sources, ensuring consistent versions for reproducibility. "+
							"Strengthen your security with vulnerability reporting and air-gapped deployments."),
				),
			),
		),
	)
}

func productCard(logoSrc string, productName string, domainPrefix string, baseUrl string, description string) Node {
	return A(
		Href(fmt.Sprintf("https://%s.%s", domainPrefix, baseUrl)),
		Target("_blank"),
		Rel("noopener noreferrer"),
		Class("block bg-white border border-gray-200 rounded-md p-6 text-center hover:bg-[#E8F1FB] transition-colors dark:bg-neutral-800 dark:border-neutral-700 dark:hover:bg-neutral-700"),
		Div(
			Class("flex justify-center mb-4"),
			Img(Class("h-6"),
				Src(logoSrc),
				Alt(productName)),
		),
		P(Class("text-sm text-gray-600 dark:text-gray-300 leading-relaxed"), Text(description)),
	)
}
