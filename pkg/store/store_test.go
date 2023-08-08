package store

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/dccn-tg/tg-toolset-golang/project/pkg/pdb"
)

func TestKVStoreSetGetObject(t *testing.T) {
	store := KVStore{
		Path: "/tmp/testKVStoreSetGetObject.db",
	}

	err := store.Connect()
	if err != nil {
		t.Errorf("%s", err)
	}
	defer store.Disconnect()

	err = store.Init([]string{"users"})
	if err != nil {
		t.Errorf("%s", err)
	}

	u1 := &pdb.User{
		ID:        "1",
		Firstname: "Hurng-Chun",
		Lastname:  "Lee",
		Email:     "h.lee@donders.ru.nl",
	}

	k := []byte(u1.ID)
	v, _ := json.Marshal(u1)

	// Set u1 to bucket "users" with ID as the key
	err = store.Set("users", k, v)
	if err != nil {
		t.Errorf("%s", err)
	}

	// Get u2 from bucket "users" with key == u1.ID.
	u2 := &pdb.User{}
	vg, err := store.Get("users", k)
	if err != nil {
		t.Errorf("%s", err)
	}
	err = json.Unmarshal(vg, u2)
	if err != nil {
		t.Errorf("%s", err)
	}

	// comparing u1 and u2
	t.Logf("u1: %+v", u1)
	t.Logf("u2: %+v", u2)
	if !reflect.DeepEqual(u1, u2) {
		t.Errorf("u1 != u2")
	}
}

func TestKVStoreGetAllObjects(t *testing.T) {
	store := KVStore{
		Path: "/tmp/testKVStoreGetAllObjects.db",
	}

	err := store.Connect()
	if err != nil {
		t.Errorf("%s", err)
	}
	defer store.Disconnect()

	err = store.Init([]string{"users"})
	if err != nil {
		t.Errorf("%s", err)
	}

	// set user entries
	u1 := &pdb.User{
		ID:        "1",
		Firstname: "Hurng-Chun",
		Lastname:  "Lee",
		Email:     "h.lee@donders.ru.nl",
	}

	k := []byte(u1.ID)
	v, _ := json.Marshal(u1)

	// Set u1 to bucket "users" with ID as the key
	err = store.Set("users", k, v)
	if err != nil {
		t.Errorf("%s", err)
	}

	// Get all users from bucket "users"
	kvpairs, err := store.GetAll("users")
	if err != nil {
		t.Errorf("%s", err)
	}

	// Check size
	if len(kvpairs) != 1 {
		t.Error("size mismatch on returned kvpairs")
	}

	// Check content
	u2 := &pdb.User{}
	vg := kvpairs[0].Value
	err = json.Unmarshal(vg, u2)
	if err != nil {
		t.Errorf("%s", err)
	}

	// comparing u1 and u2
	t.Logf("u1: %+v", u1)
	t.Logf("u2: %+v", u2)
	if !reflect.DeepEqual(u1, u2) {
		t.Errorf("u1 != u2")
	}
}
