package runtime

import (
	"errors"
	"reflect"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

func TestOwnedMenuItemLifetimeAndValues(t *testing.T) {
	title, value, removed := "Sound", 0, 0
	driver := MenuDriver{
		Title:    func(uintptr) string { return title },
		SetTitle: func(_ uintptr, next string) { title = next },
		Value:    func(uintptr) int { return value },
		SetValue: func(_ uintptr, next int) { value = next },
		Remove:   func(uintptr) { removed++ },
	}
	item := NewOwnedOptionsMenuItem(7, 2, driver)
	if err := item.SetTitle("Audio"); err != nil || item.Title() != "Audio" {
		t.Fatalf("title = %q, err = %v", item.Title(), err)
	}
	if err := item.SetValue(1); err != nil || item.Value() != 1 {
		t.Fatalf("value = %d, err = %v", item.Value(), err)
	}
	if err := item.SetValue(2); !errors.Is(err, playdate.ErrMenuValue) {
		t.Fatalf("error = %v", err)
	}
	item.Remove()
	item.Remove()
	if removed != 1 {
		t.Fatalf("removed = %d", removed)
	}
}

type forwardingMenuContext struct {
	testContext
	calls []string
	items []*forwardingMenuItem
}
type forwardingMenuItem struct {
	removed bool
	title   string
	value   int
}

func (item *forwardingMenuItem) Title() string               { return item.title }
func (item *forwardingMenuItem) SetTitle(value string) error { item.title = value; return nil }
func (item *forwardingMenuItem) Remove()                     { item.removed = true }
func (item *forwardingMenuItem) Value() int                  { return item.value }
func (item *forwardingMenuItem) SetValue(value int) error    { item.value = value; return nil }

type forwardingCheckItem struct{ *forwardingMenuItem }

func (item *forwardingCheckItem) Value() bool { return item.forwardingMenuItem.value != 0 }
func (item *forwardingCheckItem) SetValue(value bool) {
	if value {
		item.forwardingMenuItem.value = 1
	} else {
		item.forwardingMenuItem.value = 0
	}
}
func (context *forwardingMenuContext) AddActionMenuItem(title string, _ func()) (playdate.MenuItem, error) {
	item := &forwardingMenuItem{title: title}
	context.calls = append(context.calls, "action:"+title)
	context.items = append(context.items, item)
	return item, nil
}
func (context *forwardingMenuContext) AddCheckmarkMenuItem(title string, value bool, _ func()) (playdate.CheckmarkMenuItem, error) {
	item := &forwardingMenuItem{title: title}
	if value {
		item.value = 1
	}
	context.calls = append(context.calls, "check:"+title)
	context.items = append(context.items, item)
	return &forwardingCheckItem{item}, nil
}
func (context *forwardingMenuContext) AddOptionsMenuItem(title string, _ []string, _ func()) (playdate.OptionsMenuItem, error) {
	item := &forwardingMenuItem{title: title}
	context.calls = append(context.calls, "options:"+title)
	context.items = append(context.items, item)
	return item, nil
}
func (*forwardingMenuContext) Language() playdate.Language { return playdate.LanguageJapanese }
func (*forwardingMenuContext) LocalizedText(key string, _ playdate.Language) (string, bool) {
	return "localized:" + key, true
}

type forwardingMenuGame struct{}

func (forwardingMenuGame) Init(context playdate.Context) error {
	menu := any(context).(playdate.SystemMenu)
	localization := any(context).(playdate.Localization)
	_, _ = menu.AddActionMenuItem("Reset", nil)
	_, _ = menu.AddCheckmarkMenuItem("Sound", true, nil)
	_, _ = menu.AddOptionsMenuItem("Mode", []string{"A", "B"}, nil)
	if localization.Language() != playdate.LanguageJapanese {
		return errors.New("language not forwarded")
	}
	if value, ok := localization.LocalizedText("mode", playdate.LanguageSystem); !ok || value != "localized:mode" {
		return errors.New("text not forwarded")
	}
	return nil
}
func (forwardingMenuGame) Update(playdate.Context) (bool, error) { return false, nil }

func TestNewApplicationForwardsMenuLocalizationAndCleansUp(t *testing.T) {
	context := &forwardingMenuContext{}
	application, err := NewApplication(forwardingMenuGame{}, context, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := application.Handle(EventInit, 0); err != nil {
		t.Fatal(err)
	}
	if want := []string{"action:Reset", "check:Sound", "options:Mode"}; !reflect.DeepEqual(context.calls, want) {
		t.Fatalf("calls = %v", context.calls)
	}
	if err := application.Handle(EventTerminate, 0); err != nil {
		t.Fatal(err)
	}
	for _, item := range context.items {
		if !item.removed {
			t.Fatal("menu item was not removed on termination")
		}
	}
}
