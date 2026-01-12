package html

import (
	"fmt"
	"time"

	"github.com/maragudk/gomponents-heroicons/v2/outline"
	"github.com/posit-dev/team-operator/flightdeck/internal"
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/components"
	. "maragu.dev/gomponents/html"
)

func page(title string, config *internal.ServerConfig, children ...Node) Node {
	return HTML5(HTML5Props{
		Title:    fmt.Sprintf("%s - PTD", title),
		Language: "en",
		Head: []Node{
			Link(Rel("preconnect"), Href("https://fonts.googleapis.com")),
			Link(Rel("preconnect"), Href("https://fonts.gstatic.com"), Attr("crossorigin", "")),
			Link(Rel("stylesheet"), Href("https://fonts.googleapis.com/css2?family=Open+Sans:wght@400;600;700&display=swap")),
			Script(Src("https://cdn.tailwindcss.com?plugins=typography")),
			Script(Raw(`tailwind.config = {
				darkMode: 'class',
				theme: {
					fontFamily: {
						sans: ['Open Sans', 'Arial', 'Helvetica', 'sans-serif']
					}
				}
			}`)),
			Script(Raw(`
				if (localStorage.theme === 'dark' || (!('theme' in localStorage) && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
					document.documentElement.classList.add('dark')
				} else {
					document.documentElement.classList.remove('dark')
				}
			`)),
		},
		Body: []Node{
			Class("min-h-screen flex flex-col bg-gray-100 dark:bg-black font-sans"),
			Div(
				Class("flex-grow"),
				navbar(config.ShowConfig),
				Group(children),
			),
			footer(),
		},
	})
}

func footer() Node {
	return Footer(
		Class("bg-gray-100 dark:bg-black py-6 mt-auto"),
		Div(
			Class("container mx-auto px-4"),
			Div(
				Class("flex justify-between items-center"),
				Div(
					Class("flex items-center space-x-6 text-sm text-gray-500 dark:text-gray-400"),
					Span(Text(fmt.Sprintf("© %d Posit Software, PBC", time.Now().Year()))),
					A(Class("hover:text-gray-700 dark:hover:text-gray-300"), Href("https://posit.co/about/privacy-policy/"), Text("Privacy")),
					A(Class("hover:text-gray-700 dark:hover:text-gray-300"), Href("https://posit.co/about/eula/"), Text("Terms")),
					A(Class("hover:text-gray-700 dark:hover:text-gray-300"), Href("https://status.posit.co/"), Text("Status")),
				),
				A(
					Href("https://posit.co"),
					Img(Class("block dark:hidden h-6"), Src("/static/logo-team-light.svg"), Alt("Posit")),
					Img(Class("hidden dark:block h-6"), Src("/static/logo-team-dark.svg"), Alt("Posit")),
				),
			),
		),
	)
}

func navbar(showConfig bool) Node {
	return Header(
		Class("bg-white dark:bg-neutral-600 border-b border-gray-600"),
		Div(
			Class("container mx-auto py-2"),
			Nav(
				Div(Class("flex justify-between items-center"),
					A(
						Img(Class("block dark:hidden h-12"), Src("/static/logo-team-light.svg"), Alt("Posit Team")),
						Img(Class("hidden dark:block h-12"), Src("/static/logo-team-dark.svg"), Alt("Posit Team")),
						Href("/"),
					),
					Div(Class("flex justify-right space-x-4 items-center"),
						Button(
							Type("button"),
							Class("h-6 w-6 dark:text-white hover:text-gray-600 dark:hover:text-gray-500 cursor-pointer"),
							ID("theme-toggle"),
							Attr("aria-label", "Toggle theme"),
							Span(Class("hidden dark:block"), outline.Sun()),
							Span(Class("block dark:hidden"), outline.Moon()),
						),
						If(showConfig, A(Class("h-6 w-6 dark:text-white hover:text-gray-600 dark:hover:text-gray-500"), outline.Cog(), Href("/config"), TitleAttr("Configuration"))),
						A(Class("h-6 w-6 dark:text-white hover:text-gray-600 dark:hover:text-gray-500"), outline.QuestionMarkCircle(), Href("/help"), TitleAttr("Help")),
					),
				),
				Script(Raw(`
					document.getElementById('theme-toggle').addEventListener('click', function() {
						if (document.documentElement.classList.contains('dark')) {
							document.documentElement.classList.remove('dark')
							localStorage.theme = 'light'
						} else {
							document.documentElement.classList.add('dark')
							localStorage.theme = 'dark'
						}
					})
				`)),
			),
		),
	)
}
