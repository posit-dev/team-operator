package html

import (
	"github.com/posit-dev/team-operator/flightdeck/internal"
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

func HelpPage(config *internal.ServerConfig) Node {
	return page("Help", config,
		Main(
			Class("container mx-auto py-8"),
			Div(
				Class("text-center"),
				H2(Text("Send us a message!"), Class("text-lg font-bold text-gray-800 dark:text-white")),
				P(Text("We are here to help you with any questions or issues you might have. Please email us at "),
					A(
						Href("mailto:ptd@positpbc.atlassian.net"),
						Text("ptd@positpbc.atlassian.net"),
						Class("hover:underline text-blue-500 dark:text-blue-400"),
					),
					Class("text-gray-600 dark:text-gray-300")),
			),
		),
	)
}
