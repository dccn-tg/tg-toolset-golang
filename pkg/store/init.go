package store

import (
	"fmt"
	"sync"

	bolt "go.etcd.io/bbolt"
)

// KVPair is a set of key-value pair.
type KVPair struct {
	Key   []byte
	Value []byte
}

// KVStore provides interface to interact with a local database for
// managing/bookkeeping user map and exported collections.
type KVStore struct {
	Path  string
	mutex sync.Mutex
	db    *bolt.DB
}

// Connect establishes the bolt db connection.
func (s *KVStore) Connect() (err error) {
	if s.db != nil {
		return nil
	}

	if s.db, err = bolt.Open(s.Path, 0600, nil); err != nil {
		return fmt.Errorf("cannot connect blot db: %s", err)
	}
	return nil
}

// Disconnect closes the bold db connection.
func (s *KVStore) Disconnect() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Init initialize given buckets in the BOLT key-value store.
func (s *KVStore) Init(buckets []string) error {

	if s.db == nil {
		return fmt.Errorf("no connected db")
	}

	// initialize buckets if they don't exist
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, bucket := range buckets {
		if err := s.db.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte(bucket))
			if err != nil {
				return fmt.Errorf("create bucket: %s", err)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// Get returns a value of the given key within the given bucket
// in the bolt database.
func (s *KVStore) Get(bucket string, key []byte) ([]byte, error) {

	if s.db == nil {
		return nil, fmt.Errorf("no connected db")
	}

	var v []byte

	if err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		v = b.Get(key)
		if v == nil {
			return fmt.Errorf("key %+v not in bucket %s", key, bucket)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return v, nil
}

// GetAll retrieves all key-value pairs from a bucket.
func (s *KVStore) GetAll(bucket string) ([]KVPair, error) {

	if s.db == nil {
		return nil, fmt.Errorf("no connected db")
	}

	var data []KVPair

	if err := s.db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(bucket))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			data = append(data, KVPair{
				Key:   k,
				Value: v,
			})
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return data, nil
}

// Set insert/update a key-value pair in the given bucket.
func (s *KVStore) Set(bucket string, key []byte, value []byte) error {

	if s.db == nil {
		return fmt.Errorf("no connected db")
	}

	// initialize buckets if they don't exist
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if err := b.Put([]byte(key), value); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}
