package main

import (
	"context"
	"image"
	"image/color"
	"log"
	"os"
	"os/signal"
	"syscall"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/Niceblueman/CloneMe/dataset"
	_ "github.com/mattn/go-sqlite3"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// QRCodeWidget is a custom Fyne widget for displaying a QR code.

type QRCodeWidget struct {
	*canvas.Image
}

var client *whatsmeow.Client
var _app fyne.App
var _tab *container.TabItem
var _mainwin fyne.Window
var _appconfig dataset.Config = dataset.Config{}
var _messages dataset.Messages = dataset.Messages{}

// NewQRCodeWidget creates a new instance of the QRCodeWidget.
func NewQRCodeWidget() *QRCodeWidget {
	widget := &QRCodeWidget{
		Image: canvas.NewImageFromImage(image.NewRGBA(image.Rect(1, 0, 1, 1))),
	}
	widget.SetMinSize(fyne.NewSquareSize(300))
	widget.FillMode = canvas.ImageFillOriginal
	return widget
}

// SetQRCode sets the QR code data for the widget.
func (q *QRCodeWidget) SetQRCode(data string) error {
	qr, err := qrcode.New(data, qrcode.Medium)
	if err != nil {
		return err
	}
	q.Image.Image = qr.Image(256)
	q.Refresh()
	q.Image.Refresh()
	return nil
}

func EventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		if v.Info.IsFromMe {
			msg := v.Message.GetExtendedTextMessage()
			if msg != nil {
				_messages.Upsert(dataset.Message{
					CustID:   v.Info.ID,
					Role:     "assistant",
					Content:  msg.GetText(),
					Selected: false,
				})
			}
		} else {
			msg := v.Message.GetExtendedTextMessage()
			if msg != nil {
				_messages.Upsert(dataset.Message{
					CustID:   v.Info.ID,
					Role:     "user",
					Content:  msg.GetText(),
					Selected: false,
				})
			}
			_tab.Content = GetMessagesTable()
		}
	default:
	}
}

func main() {
	dbLog := waLog.Stdout("Database", "DEBUG", true)
	_container, err := sqlstore.New("sqlite3", "file:accounts.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}
	_appconfig.New()
	_messages.New()
	// If you want multiple sessions, remember their JIDs and use .GetDevice(jid) or .GetAllDevices() instead.
	deviceStore, err := _container.GetFirstDevice()
	if err != nil {
		panic(err)
	}
	clientLog := waLog.Stdout("Client", "ERROR", true)
	client = whatsmeow.NewClient(deviceStore, clientLog)
	_app = app.NewWithID("cloneme.dup.company")
	_mainwin := _app.NewWindow("Login")
	qrWidget := NewQRCodeWidget()
	// Create a center layout
	text := canvas.NewText("scan Qrcode to login", color.White)
	centerLayout := container.New(layout.NewCenterLayout(),
		container.New(layout.NewVBoxLayout(),
			qrWidget.Image,
			container.New(layout.NewCenterLayout(),
				text,
			),
		),
	)
	go func() {
		if client.Store.ID == nil {
			// No ID stored, new login
			qrChan, _ := client.GetQRChannel(context.Background())
			err = client.Connect()
			if err != nil {
				log.Println(err)
			}
			for evt := range qrChan {
				if evt.Event == "code" {
					// Render the QR code here
					// e.g. qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
					// or just manually `echo 2@... | qrencode -t ansiutf8` in a terminal
					qrWidget.SetQRCode(evt.Code)
				} else {
					showTabs(_mainwin)
					client.AddEventHandler(EventHandler)
				}
			}
		} else {
			// Already logged in, just connect
			err = client.Connect()
			showTabs(_mainwin)
			client.AddEventHandler(EventHandler)
			if err != nil {
				log.Println(err)
			}
		}
		// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		client.Disconnect()
	}()
	_mainwin.SetContent(centerLayout)
	_mainwin.Resize(fyne.NewSize(700, 600)) // Adjust window size as needed
	_mainwin.ShowAndRun()
}

func showTabs(w fyne.Window) {
	w.SetTitle("CloneMe: Dashboard")
	_tab = container.NewTabItemWithIcon("Messages", resourceIcons8Whatsapp64Png, GetMessagesTable())
	clonerTab := container.NewTabItemWithIcon("Cloner", resourceIcons8Settings64Png, GetClonerForm())
	tabs := container.NewAppTabs(_tab, clonerTab)
	tabs.SetTabLocation(container.TabLocationLeading)

	// Set up UI layout
	w.SetContent(tabs)
	w.RequestFocus()
}
func GetMessagesTable() fyne.CanvasObject {
	data := []dataset.Message{}
	if _data, err := _messages.Load(); err == nil {
		data = append(data, _data...)
	}
	return dataset.NewDataTable(data)
}
func GetClonerForm() *fyne.Container {
	// Create input fields
	cureconfigs, _ := _appconfig.Load()
	var groupsoptions []*types.GroupInfo
	activegroup := ""
	var err error
	if client != nil {
		groupsoptions, err = client.GetJoinedGroups()
		if err != nil {
			groupsoptions = []*types.GroupInfo{}
		}
	}
	_options := []string{}
	activegroup = ""
	if len(groupsoptions) > 0 {
		activegroup = groupsoptions[0].Name
	}
	for _, info := range groupsoptions {
		_options = append(_options, info.Name)
		if cureconfigs.Groupid != "" && cureconfigs.Groupid == info.JID.String() {
			activegroup = info.Name
		}
	}
	openAIKeyEntry := widget.NewEntry()
	if cureconfigs.OpenAIKey != "" {
		openAIKeyEntry.SetText(cureconfigs.OpenAIKey)
		openAIKeyEntry.Refresh()
	}
	openAIKeyEntry.SetPlaceHolder("Enter OpenAI Key")

	assistantContextEntry := widget.NewEntry()
	assistantContextEntry.TextStyle = fyne.TextStyle{
		Symbol:    true,
		Monospace: true,
	}
	assistantContextEntry.MultiLine = true
	assistantContextEntry.Wrapping = fyne.TextWrapWord
	assistantContextEntry.SetMinRowsVisible(6)
	if cureconfigs.AssistantContext != "" {
		assistantContextEntry.SetText(cureconfigs.AssistantContext)
		assistantContextEntry.Refresh()
	}
	assistantContextEntry.SetPlaceHolder("Enter Assistant Context")

	groupSelect := widget.NewSelect(_options, nil)
	groupSelect.SetSelected(activegroup)
	groupSelect.Refresh()
	// Create form
	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "OpenAI Key", Widget: openAIKeyEntry},
			{Text: "Assistant Context", Widget: assistantContextEntry},
			{Text: "Select Group", Widget: groupSelect},
		},
		BaseWidget: widget.BaseWidget{},
		OnSubmit: func() {
			// Handle form submission here
			// You can access the input values using openAIKeyEntry.Text(), assistantContextEntry.Text, and groupSelect.Selected
			selected := ""
			for _, info := range groupsoptions {
				if info.Name == groupSelect.Selected {
					selected = info.JID.String()
				}
			}
			dialogwin := _app.NewWindow("dialog")
			dialogwin.SetFixedSize(true)
			dialogwin.Resize(fyne.NewSize(250, 150))
			dialogwin.SetIcon(fyne.CurrentApp().Icon())
			dialog.ShowConfirm("Save", "Confirm and save new configs", func(status bool) {
				if status {
					if _, err := _appconfig.Upsert(dataset.Configs{
						CustID:           "app",
						OpenAIKey:        openAIKeyEntry.Text,
						AssistantContext: assistantContextEntry.Text,
						Groupid:          selected,
					}); err != nil {
						_app.SendNotification(fyne.NewNotification("Error", err.Error()))
					}
				}
				dialogwin.Hide()
			}, dialogwin)
			dialogwin.Show()
		},
	}
	form.SubmitText = "Save"
	// Center the form vertically and horizontally
	centeredLayout := container.New(layout.NewStackLayout(), form)
	return centeredLayout
}
