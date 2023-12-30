package gui

import (
	"fmt"
	"net/url"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"github.com/ubavic/bas-celik/document"
	"github.com/ubavic/bas-celik/gui/widgets"
)

type State struct {
	mu          sync.Mutex
	startPageOn bool
	verbose     bool
	window      *fyne.Window
	startPage   *widgets.StartPage
	toolbar     *widgets.Toolbar
	spacer      *widgets.Spacer
	statusBar   *widgets.StatusBar
}

var state State

var application fyne.App

func StartGui(verbose_ bool, version string) {
	application = app.NewWithID("com.github.ubavic.bas_celik")
	win := application.NewWindow("Baš Čelik")
	application.Settings().SetTheme(MyTheme{})

	showAboutBox := ShowAboutBox(win, version)
	showSettingsBox := ShowSettingsBox(win, version)

	statusBar := widgets.NewStatusBar()
	toolbar := widgets.NewToolbar(showAboutBox, showSettingsBox)
	spacer := widgets.NewSpacer()
	startPage := widgets.NewStartPage()
	startPage.SetStatus("", "", false)

	state = State{
		startPageOn: true,
		verbose:     verbose_,
		toolbar:     toolbar,
		startPage:   startPage,
		window:      &win,
		spacer:      spacer,
		statusBar:   statusBar,
	}

	rows := container.New(layout.NewVBoxLayout(), toolbar, spacer, startPage)
	win.SetContent(container.New(layout.NewPaddedLayout(), rows))

	go pooler()

	win.ShowAndRun()
}

func setUI(doc document.Document) {
	state.mu.Lock()
	defer state.mu.Unlock()

	pdfHandler := savePdf(doc)
	saveButton := widget.NewButton("Sačuvaj PDF", pdfHandler)
	buttonBar := container.New(layout.NewHBoxLayout(), state.statusBar, layout.NewSpacer(), saveButton)

	var page *fyne.Container
	switch doc := doc.(type) {
	case *document.IdDocument:
		page = pageID(doc)
	case *document.MedicalDocument:
		page = pageMedical(doc)
	case *document.VehicleDocument:
		page = pageVehicle(doc)
	}

	rows := container.New(layout.NewVBoxLayout(), state.toolbar, state.spacer, page, buttonBar)
	columns := container.New(layout.NewHBoxLayout(), layout.NewSpacer(), rows, layout.NewSpacer())
	container := container.New(layout.NewPaddedLayout(), columns)
	(*state.window).SetContent(container)

	(*state.window).Resize(container.MinSize())
	state.startPageOn = false
}

func setStartPage(status, explanation string, err error) {
	state.mu.Lock()
	defer state.mu.Unlock()

	isError := false
	if err != nil {
		isError = true
	}

	if state.verbose && isError {
		fmt.Println(err)
	}

	state.startPage.SetStatus(status, explanation, isError)
	state.startPage.Refresh()

	if !state.startPageOn {
		rows := container.New(layout.NewVBoxLayout(), state.toolbar, state.spacer, state.startPage, layout.NewSpacer())
		(*state.window).SetContent(container.New(layout.NewPaddedLayout(), rows))
		state.startPageOn = true
	}
}

func setStatus(status string, err error) {
	isError := false
	if err != nil {
		isError = true
	}

	if state.verbose && isError {
		fmt.Println(err)
	}

	state.statusBar.SetStatus(status, isError)
	state.statusBar.Refresh()
}

func setApiStatus(status string) {
	state.statusBar.SetApiStatus(status)
	state.statusBar.Refresh()
}

func savePdf(doc document.Document) func() {
	return func() {
		pdf, fileName, err := doc.BuildPdf()

		if err != nil {
			setStatus("Greška pri generisanju PDF-a", fmt.Errorf("generating PDF: %w", err))
			return
		}

		dialog := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
			if w == nil || err != nil {
				return
			}

			_, err = w.Write(pdf)
			if err != nil {
				setStatus("Greška pri zapisivanju PDF-a", fmt.Errorf("writing PDF: %w", err))
				return
			}

			err = w.Close()
			if err != nil {
				setStatus("Greška pri zapisivanju PDF-a", fmt.Errorf("writing PDF: %w", err))
				return
			}

			setStatus("PDF sačuvan", nil)
		}, *state.window)

		dialog.SetFilter(storage.NewExtensionFileFilter([]string{".pdf"}))
		dialog.SetFileName(fileName)

		dialog.Show()
	}
}

func ShowAboutBox(win fyne.Window, version string) func() {
	verLabel := widget.NewLabelWithStyle("Verzija: "+version, fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
	moreLabel := widget.NewLabel("Više o programu na adresi:")
	url, _ := url.Parse("https://github.com/ubavic/bas-celik")
	linkLabel := widget.NewHyperlink("github.com/ubavic/bas-celik", url)
	spacer := widgets.NewSpacer()
	hBox := container.NewHBox(moreLabel, linkLabel)
	vBox := container.NewVBox(verLabel, hBox, spacer)

	return func() {
		dialog.ShowCustom(
			"Baš Čelik - program za očitavanje elektronskih dokumenata",
			"Zatvori",
			vBox,
			win,
		)
	}
}

func ShowSettingsBox(win fyne.Window, version string) func() {

	apiURLLabel := widget.NewLabel("API URL:")
	apiURL := widget.NewEntry()
	apiURL.SetText(application.Preferences().String("apiURL"))
	apiURL.OnChanged = func(s string) {
		application.Preferences().SetString("apiURL", s)
	}

	apiKeyLabel := widget.NewLabel("API Key:")
	apiKey := widget.NewEntry()
	apiKey.SetText(application.Preferences().String("apiKey"))
	apiKey.OnChanged = func(s string) {
		application.Preferences().SetString("apiKey", s)
	}

	grid := container.New(layout.NewFormLayout(), apiURLLabel, apiURL, apiKeyLabel, apiKey)

	return func() {
		dialog.ShowCustom(
			"Baš Čelik - Podešavanja                                                        ",
			"Zatvori",
			grid,
			win,
		)
	}
}
