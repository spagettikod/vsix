package vscode

import (
	"strings"
)

type UniqueID struct {
	Publisher string
	Name      string
}

func (uid UniqueID) IsZero() bool {
	return uid.Publisher == "" || uid.Name == ""
}

func (uid UniqueID) Valid() bool {
	return !uid.IsZero()
}

func (uid UniqueID) String() string {
	return uid.Publisher + "." + uid.Name
}

func (uid UniqueID) Equals(uid2 UniqueID) bool {
	return uid.String() == uid2.String()
}

func Parse(id string) (UniqueID, bool) {
	if id == "" {
		return UniqueID{}, false
	}
	spl := strings.Split(id, ".")
	if len(spl) != 2 {
		return UniqueID{}, false
	}
	return UniqueID{Publisher: spl[0], Name: spl[1]}, true
}
