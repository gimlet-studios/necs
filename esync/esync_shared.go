package esync

import (
	"reflect"
	"unsafe"

	"github.com/leap-fish/necs/typemapper"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/filter"
)

var InterpComponent = donburi.NewComponentType[InterpData]()

type InterpData struct {
	Components []uint
}

func NewInterpData(components ...donburi.IComponentType) *InterpData {
	ids := []uint{}
	for i := range components {
		ids = append(ids, interpolated.LookupId(components[i].Typ()))
	}

	return &InterpData{
		Components: ids,
	}
}

func (i *InterpData) ComponentKeys() []uint {
	return i.Components
}

type ComponentId uint
type NetworkId uint

type EntityState map[ComponentId][]byte
type SerializedEntity struct {
	Id    NetworkId
	State EntityState
}
type WorldSnapshot []SerializedEntity

// LerpFn is used by the InterpolateSystem to properly lerp your component
type LerpFn[T any] func(from T, to T, delta float64) *T

var NetworkEntityQuery = donburi.NewQuery(filter.Contains(NetworkIdComponent))

var interpolated = typemapper.NewComponentMapper()
var registered = map[reflect.Type]donburi.IComponentType{}

var (
	Mapper             = typemapper.NewMapper(map[uint]any{})
	NetworkIdComponent = donburi.NewComponentType[NetworkId]()
)

// RegisterInterpolated creates a contract that the client and server understand
// by assigning it an ID that the client knows to interpolate your passed component type
// as well as how to interpolate the component values by passing the lerp function.
//
// For example you can provide the following for a basic Vector2:
//
//	var PositionComponent = donburi.NewComponentType[Vector2]()
//
//	type Vector2 struct {
//		X, Y float64
//	}
//
//	func lerp(a, b, t float64) float64 {
//		return (1.0-t)*a + b*t
//	}
//
//	esync.RegisterInterpolated(1, PositionComponent, func(from, to Vector2, delta float64) *Vector2 {
//		return &Vector2{
//			X: lerp(from.X, to.X, delta),
//			T: lerp(from.Y, to.Y, delta),
//		}
//	})
func RegisterInterpolated[T any](id uint, comp *donburi.ComponentType[T], lerp LerpFn[T]) error {
	return interpolated.RegisterInterpolatedComponent(id, comp, lerp)
}

// LookInterpId returns the interpolation ID for the given type, if not present
// then 0 is returned.
func LookupInterpId(typ reflect.Type) uint {
	return interpolated.LookupId(typ)
}

// LookupInterpType returns the component type for the given interpolation ID,
// if not present then an empty reflect.Type is returned.
func LookupInterpType(id uint) reflect.Type {
	return interpolated.LookupType(id)
}

// LookupInterpSetter returns the setter function for the given interpolation ID.
// This should always be type [esync.LerpFn]
func LookupInterpSetter(id uint) any {
	return interpolated.LookupSetter(id)
}

// RegisteredInterpId returns true if the given interpolation ID is registered.
func RegisteredInterpId(id uint) bool {
	return interpolated.RegisteredId(id)
}

// RegisteredInterpType returns true if the given component type is registered for
// interpolation.
func RegisteredInterpType(typ reflect.Type) bool {
	return interpolated.RegisteredType(typ)
}

func Registered(componentType reflect.Type) (donburi.IComponentType, bool) {
	ctype, ok := registered[componentType]
	return ctype, ok
}

// RegisterComponent registers a component for use with esync. Make sure the client and server have the same definition of components.
// Note that ID 1 is reserved for the NetworkId component used by esync.
func RegisterComponent(id uint, component any, ctype donburi.IComponentType) error {
	typ := reflect.TypeOf(component)
	err := Mapper.RegisterType(id, typ)
	if err != nil {
		return err
	}
	registered[typ] = ctype

	return nil
}

// FindByNetworkId performs an "Each" query over network entities to find one with a matching ID.
func FindByNetworkId(world donburi.World, networkId NetworkId) donburi.Entity {
	var found donburi.Entity
	NetworkEntityQuery.Each(world, func(entry *donburi.Entry) {
		id := GetNetworkId(entry)
		if id == nil || *id != networkId {
			return
		}

		found = entry.Entity()
	})

	return found
}

func GetNetworkId(entry *donburi.Entry) *NetworkId {
	if entry == nil {
		return nil
	}

	if !entry.Valid() {
		return nil
	}

	if !entry.HasComponent(NetworkIdComponent) {
		return nil
	}

	nid := NetworkIdComponent.Get(entry)
	return nid
}

func ComponentFromVal(ctype donburi.IComponentType, value interface{}) unsafe.Pointer {
	if reflect.TypeOf(value) != ctype.Typ() {
		panic("Type assertion failed")
	}
	newVal := reflect.New(ctype.Typ()).Elem()
	newVal.Set(reflect.ValueOf(value))
	ptr := unsafe.Pointer(newVal.UnsafeAddr())

	return ptr
}
