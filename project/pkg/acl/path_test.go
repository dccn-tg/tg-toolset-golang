package acl

import (
    "testing"
)

func TestAddPathForTraverse(t *testing.T) {
    
    paths4traverse := []string{}
    
    expected1 := map[string]bool {
        "/a"     : true,
        "/a/b"   : true,
        "/a/b/c" : true,
    }
    
    expected2 := map[string]bool {
        "/a"       : true,
        "/a/b"     : true,
        "/a/b/c"   : true,
        "/a/b/c/e" : true,
    }
    
    p1 := "/a/b/c/d.txt"
    AddPathForTraverse(p1, &paths4traverse)
    // here we expect paths4traverse contains "/a", "/a/b", "/a/b/c"
    if len(paths4traverse) != len(expected1) {
        t.Errorf("Expected paths4traverse size %d but got %d", len(expected1), len(paths4traverse))
    }
    for _,p := range paths4traverse {
        if ! expected1[p] {
            t.Errorf("Unexpected path: %s", p)
        }
    }
    
    p2 := "/a/b/c/t.txt"
    AddPathForTraverse(p2, &paths4traverse)
    // here we expect paths4traverse contains also "/a", "/a/b", "/a/b/c"
    if len(paths4traverse) != len(expected1) {
        t.Errorf("Expected paths4traverse size %d but got %d", len(expected1), len(paths4traverse))
    }
    for _,p := range paths4traverse {
        if ! expected1[p] {
            t.Errorf("Unexpected path: %s", p)
        }
    }
        
    p3 := "/a/b/c/e/x.txt"
    AddPathForTraverse(p3, &paths4traverse)
    // here we expect paths4traverse contains also "/a", "/a/b", "/a/b/c", "/a/b/c/d"
    if len(paths4traverse) != len(expected2) {
        t.Errorf("Expected paths4traverse size %d but got %d", len(expected2), len(paths4traverse))
    }
    for _,p := range paths4traverse {
        if ! expected2[p] {
            t.Errorf("Unexpected path: %s", p)
        }
    }
}