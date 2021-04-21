package activitylog

// This file is auto-generated.
//
// Changes to this file may cause incorrect behavior and will be lost if
// the code is regenerated.
//
// Definitions file that controls how this file is generated:
// pkg/activitylog/types.yaml

type (

	// ActivitySet slice of Activity
	//
	// This type is auto-generated.
	ActivitySet []*Activity
)

// Walk iterates through every slice item and calls w(Activity) err
//
// This function is auto-generated.
func (set ActivitySet) Walk(w func(*Activity) error) (err error) {
	for i := range set {
		if err = w(set[i]); err != nil {
			return
		}
	}

	return
}

// Filter iterates through every slice item, calls f(Activity) (bool, err) and return filtered slice
//
// This function is auto-generated.
func (set ActivitySet) Filter(f func(*Activity) (bool, error)) (out ActivitySet, err error) {
	var ok bool
	out = ActivitySet{}
	for i := range set {
		if ok, err = f(set[i]); err != nil {
			return
		} else if ok {
			out = append(out, set[i])
		}
	}

	return
}

// FindByID finds items from slice by its ID property
//
// This function is auto-generated.
func (set ActivitySet) FindByID(ID uint64) *Activity {
	for i := range set {
		if set[i].ID == ID {
			return set[i]
		}
	}

	return nil
}

// IDs returns a slice of uint64s from all items in the set
//
// This function is auto-generated.
func (set ActivitySet) IDs() (IDs []uint64) {
	IDs = make([]uint64, len(set))

	for i := range set {
		IDs[i] = set[i].ID
	}

	return
}
