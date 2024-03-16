package dataset

import (
	"context"
	"fmt"
	"log"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/sashabaranov/go-openai"
	db "github.com/sonyarouje/simdb"
)

type Message struct {
	CustID   string `json:"custid"`
	Role     string `json:"role"` // user | assistant
	Content  string `json:"content"`
	Selected bool   `json:"selected"`
}
type Messages struct {
	db *db.Driver
}

func (c Message) ID() (jsonField string, value interface{}) {
	value = c.CustID
	jsonField = "custid"
	return
}
func (conf *Messages) New() error {
	var err error
	conf.db, err = db.New("jsonl")
	if err != nil {
		return err
	}
	return nil
}
func (conf *Messages) Load() ([]Message, error) {
	var messages []Message
	err := conf.db.Open(Message{}).Get().AsEntity(&messages)
	if err != nil {
		log.Println(err)
		return []Message{}, err
	}
	return messages, nil
}

func (conf *Messages) Upsert(input Message) (Message, error) {
	err := conf.db.Upsert(input)
	if err != nil {
		return Message{}, err
	}
	return input, nil
}
func (conf *Messages) Remove(id string) error {
	err := conf.db.Delete(Message{
		CustID: id,
	})
	if err != nil {
		return err
	}
	return nil
}

type Dataset struct {
	client *openai.Client
	db     openai.AssistantFile
	sync.Mutex
}

func (dset *Dataset) New() error {
	var err error
	config := Config{}
	appconfig, err := config.Load()
	if err != nil {
		return err
	}
	dset.client = openai.NewClient(appconfig.OpenAIKey)
	if dset.client != nil {
		limit := 1
		order := ""
		after := ""
		before := ""
		assistants, err := dset.client.ListAssistants(context.Background(), &limit, &order, &after, &before)
		if err != nil {
			return err
		}
		dset.db, err = dset.client.CreateAssistantFile(context.Background(), *assistants.FirstID, openai.AssistantFileRequest{
			FileID: "datsetnew",
		})
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("not connected")
	}
	return nil
}

type Config struct {
	db *db.Driver
}
type Configs struct {
	CustID           string `json:"custid"`
	OpenAIKey        string `json:"openapikey"`
	AssistantContext string `json:"assistantcontext"`
	Groupid          string `json:"groupid"`
}

func (c Configs) ID() (jsonField string, value interface{}) {
	value = c.CustID
	jsonField = "custid"
	return
}
func (conf *Config) New() error {
	var err error
	conf.db, err = db.New("data")
	if err != nil {
		return err
	}
	return nil
}
func (conf *Config) Load() (Configs, error) {
	var appconf Configs
	err := conf.db.Open(Configs{}).Where("custid", "=", "app").First().AsEntity(&appconf)
	if err != nil {
		log.Println(err)
		return Configs{}, err
	}
	return appconf, nil
}

func (conf *Config) Upsert(input Configs) (Configs, error) {
	err := conf.db.Upsert(input)
	if err != nil {
		return Configs{}, err
	}
	return input, nil
}

// Custom data table widget
type DataTable struct {
	*widget.List
	Data []Message
}

func NewDataTable(data []Message) fyne.CanvasObject {
	table := &DataTable{
		Data: data,
	}
	table.List = widget.NewList(
		func() int {
			return len(table.Data)
		},
		func() fyne.CanvasObject {
			return container.New(layout.NewHBoxLayout(),
				widget.NewCheck("", func(b bool) {}),
				widget.NewLabel(""),
				widget.NewLabel(""),
			)
		},
		func(i int, o fyne.CanvasObject) {
			if item := table.Data[i]; len(table.Data) > 0 {
				o.(*fyne.Container).Objects[0].(*widget.Check).SetChecked(item.Selected)
				o.(*fyne.Container).Objects[1].(*widget.Label).SetText(item.Role)
				o.(*fyne.Container).Objects[2].(*widget.Label).SetText(item.Content)
			}
		},
	)
	SPLIT := container.NewVSplit(table.List,
		container.NewHBox(layout.NewSpacer(), widget.NewButton("REFRESH", func() {
			table.List.Refresh()
			table.List.ScrollToBottom()
		})),
	)
	SPLIT.SetOffset(1)
	result := container.New(layout.NewPaddedLayout(),
		SPLIT,
	)
	return result
}

func (t *DataTable) RefreshItems() int {
	t.List.Refresh()
	return t.List.Length()
}
