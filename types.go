package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"sort"
)

type BadgerCopyArgs struct {
	Copy          bool
	AssumeYes     bool
	BlurThreshold int
	SrcDir        string
	DstDir        string
}

type Media struct {
	idx   string
	fpath string
	mtime int
}

type CopyRegistry struct {
	Fpath    string          `json:"fpath"`
	Entries  map[string]bool `json:"entries"`
	ListHash string          `json:"listHash"`
}

func SyncCopyRegistry(fpath string, clusters [][]Media) {
	reg := CopyRegistry{
		Fpath:    fpath,
		Entries:  map[string]bool{},
		ListHash: "",
	}

	fpaths := make([]string, 0)

	for _, cluster := range clusters {
		for _, media := range cluster {
			fpaths = append(fpaths, media.fpath)
		}
	}

	sort.Strings(fpaths)

	for _, fpath := range fpaths {
		reg.Entries[fpath] = false
	}

	// add to dict
	// hash
	// load
	// compare / merge
	// write
	// return
}

func (reg *CopyRegistry) JSON() ([]byte, error) {
	return json.Marshal(reg)
}

func (reg *CopyRegistry) ReadJSON() (CopyRegistry, error) {
	conn, err := os.Open(reg.Fpath)
	if err != nil {
		return CopyRegistry{}, err
	}

	byteValue, _ := ioutil.ReadAll(conn)

	var loaded CopyRegistry
	err = json.Unmarshal(byteValue, loaded)

	if err != nil {
		return CopyRegistry{}, err
	}

	return loaded, nil
}

func (reg *CopyRegistry) WriteJSON() error {
	conn, err := os.Open(reg.Fpath)
	if err != nil {
		return err
	}

	serialised, err := reg.JSON()

	if err != nil {
		return err
	}

	_, err = conn.Write(serialised)

	if err != nil {
		return err
	}

	return nil
}
