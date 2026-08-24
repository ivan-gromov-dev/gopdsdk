// Package reflection demonstrates the bounded public reflection device subset.
package reflection

import (
	"reflect"
	"runtime"
	"strconv"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

const (
	soakDurationMS = 60 * 1000
	heapGrowthMax  = 32 * 1024
)

type fixture struct {
	Count int    `pd:"count"`
	Name  string `pd:"name"`
}

type game struct {
	startMS      uint32
	started      bool
	frames       uint32
	operationsOK bool
	allocBytes   [5]uint64
	baselineHeap uint64
	maxHeap      uint64
	memoryOK     bool
	soakComplete bool
}

// New creates the bounded-reflection acceptance game.
func New() playdate.Game { return &game{} }

func (game *game) Init(playdate.Context) error {
	game.operationsOK = reflectionFixture() == 56
	operations := [...]func() bool{metadataFixture, interfaceFixture, structMutationFixture, sliceMutationFixture, mapMutationFixture}
	for index, operation := range operations {
		game.allocBytes[index] = measureAllocation(operation)
	}
	return nil
}

func (game *game) Update(context playdate.Context) (bool, error) {
	now := context.CurrentTimeMilliseconds()
	if !game.started {
		game.started = true
		game.startMS = now
	}
	if reflectionFixture() != 56 {
		game.operationsOK = false
	}
	game.frames++
	if game.frames%30 == 0 {
		runtime.GC()
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		if game.baselineHeap == 0 {
			game.baselineHeap = stats.HeapAlloc
		}
		if stats.HeapAlloc > game.maxHeap {
			game.maxHeap = stats.HeapAlloc
		}
		game.memoryOK = game.maxHeap-game.baselineHeap <= heapGrowthMax
	}
	game.soakComplete = now-game.startMS >= soakDurationMS

	context.Clear()
	context.DrawText("P12.3 reflection proof", 12, 8)
	context.DrawText("Operations "+pass(game.operationsOK), 12, 30)
	var allocated uint64
	for _, bytes := range game.allocBytes {
		allocated += bytes
	}
	context.DrawText("Alloc "+strconv.FormatUint(allocated, 10)+" bytes", 12, 52)
	context.DrawText("Memory "+pass(game.memoryOK), 12, 74)
	context.DrawText("Soak "+pass(game.soakComplete), 12, 96)
	return true, nil
}

func reflectionFixture() int {
	if !metadataFixture() || !interfaceFixture() || !structMutationFixture() || !sliceMutationFixture() || !mapMutationFixture() {
		return 0
	}
	return 56
}

func metadataFixture() bool {
	value := fixture{}
	typeInfo := reflect.TypeOf(value)
	if typeInfo.Kind() != reflect.Struct || typeInfo.Name() != "fixture" || typeInfo.NumField() != 2 {
		return false
	}
	field := typeInfo.Field(0)
	return field.Name == "Count" && field.Tag.Get("pd") == "count"
}

func interfaceFixture() bool {
	value := fixture{Count: 3}
	root := reflect.ValueOf(&value).Elem()
	count := root.Field(0)
	if !count.CanSet() || !count.CanInterface() || count.Interface().(int) != 3 {
		return false
	}
	return count.Convert(reflect.TypeOf(int64(0))).Int() == 3
}

func structMutationFixture() bool {
	value := fixture{Count: 3, Name: "old"}
	root := reflect.ValueOf(&value).Elem()
	root.Field(0).SetInt(7)
	root.Field(1).SetString("new")
	return value.Count == 7 && value.Name == "new"
}

func sliceMutationFixture() bool {
	numbers := []int{10, 20}
	slice := reflect.ValueOf(numbers)
	slice.Index(1).SetInt(30)
	return numbers[1] == 30
}

func mapMutationFixture() bool {
	values := map[string]int{"old": 1}
	mapping := reflect.ValueOf(values)
	key := reflect.ValueOf("answer")
	mapping.SetMapIndex(key, reflect.ValueOf(16))
	answer := mapping.MapIndex(key)
	if !answer.IsValid() || answer.Int() != 16 {
		return false
	}
	return values["answer"] == 16
}

func measureAllocation(operation func() bool) uint64 {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for range 32 {
		operation()
	}
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

func pass(ok bool) string {
	if ok {
		return "PASS"
	}
	return "----"
}
