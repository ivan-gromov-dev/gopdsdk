package runtime

import "github.com/ivan-gromov-dev/gopdsdk/playdate"

var menuCallbacks = struct {
	next  uint32
	items map[uint32]func()
}{items: make(map[uint32]func())}

// RegisterMenuCallback retains a callback until its native item is removed.
func RegisterMenuCallback(callback func()) uint32 {
	menuCallbacks.next++
	if menuCallbacks.next == 0 {
		menuCallbacks.next++
	}
	menuCallbacks.items[menuCallbacks.next] = callback
	return menuCallbacks.next
}

// InvokeMenuCallback invokes a callback from a native ABI trampoline.
func InvokeMenuCallback(id uint32) {
	callback := menuCallbacks.items[id]
	if callback != nil {
		callback()
	}
}

// ForgetMenuCallback releases a callback after native item removal.
func ForgetMenuCallback(id uint32) {
	delete(menuCallbacks.items, id)
}

// MenuDriver adapts owned menu items to a native ABI.
type MenuDriver struct {
	SetTitle func(uintptr, string)
	Title    func(uintptr) string
	Value    func(uintptr) int
	SetValue func(uintptr, int)
	Remove   func(uintptr)
}

type ownedMenuItem struct {
	handle  uintptr
	driver  MenuDriver
	options int
}

// NewOwnedMenuItem wraps a native item returned by an ABI adapter.
func NewOwnedMenuItem(handle uintptr, options int, driver MenuDriver) playdate.MenuItem {
	return &ownedMenuItem{handle: handle, options: options, driver: driver}
}

func (item *ownedMenuItem) Title() string {
	if item.handle == 0 {
		return ""
	}
	return item.driver.Title(item.handle)
}

func (item *ownedMenuItem) SetTitle(title string) error {
	if title == "" {
		return playdate.ErrMenuTitle
	}
	if item.handle != 0 {
		item.driver.SetTitle(item.handle, title)
	}
	return nil
}

func (item *ownedMenuItem) Remove() {
	if item.handle != 0 {
		item.driver.Remove(item.handle)
		item.handle = 0
	}
}

func (item *ownedMenuItem) Value() int {
	if item.handle == 0 {
		return 0
	}
	return item.driver.Value(item.handle)
}

func (item *ownedMenuItem) SetValue(value int) error {
	if item.options > 0 && (value < 0 || value >= item.options) {
		return playdate.ErrMenuValue
	}
	if item.handle != 0 {
		item.driver.SetValue(item.handle, value)
	}
	return nil
}

type ownedCheckmarkMenuItem struct{ *ownedMenuItem }

func (item *ownedCheckmarkMenuItem) Value() bool { return item.ownedMenuItem.Value() != 0 }
func (item *ownedCheckmarkMenuItem) SetValue(value bool) {
	native := 0
	if value {
		native = 1
	}
	_ = item.ownedMenuItem.SetValue(native)
}

// NewOwnedCheckmarkMenuItem wraps a native checkmark item.
func NewOwnedCheckmarkMenuItem(handle uintptr, driver MenuDriver) playdate.CheckmarkMenuItem {
	return &ownedCheckmarkMenuItem{&ownedMenuItem{handle: handle, options: 2, driver: driver}}
}

// NewOwnedOptionsMenuItem wraps a native options item.
func NewOwnedOptionsMenuItem(handle uintptr, count int, driver MenuDriver) playdate.OptionsMenuItem {
	return &ownedMenuItem{handle: handle, options: count, driver: driver}
}
