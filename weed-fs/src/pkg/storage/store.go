package storage

import (
	"log"
	"io/ioutil"
	"json"
	"strings"
	"strconv"
	"url"
)

type Store struct {
	volumes      map[uint64]*Volume
	capacity     int
	dir          string
	Port         int
	PublicServer string
}
type VolumeInfo struct {
	Id       uint64
	Size int64
}

func NewStore(port int, publicServer, dirname string, chunkSize, capacity int) (s *Store) {
	s = new(Store)
	s.Port, s.PublicServer, s.dir, s.capacity = port, publicServer, dirname, capacity
	s.volumes = make(map[uint64]*Volume)

	files, _ := ioutil.ReadDir(dirname)
	for _, f := range files {
		if f.IsDirectory() || !strings.HasSuffix(f.Name, ".dat") {
			continue
		}
		id, err := strconv.Atoui64(f.Name[0:(strings.LastIndex(f.Name, ".dat"))])
		log.Println("Loading data file name:", f.Name)
		if err != nil {
			continue
		}
		s.volumes[id] = NewVolume(s.dir, id)
	}
	log.Println("Store started on dir:", dirname, "with", len(s.volumes), "existing volumes")
	log.Println("Expected capacity=", s.capacity, "volumes")
	return
}

func (s *Store) Join(mserver string) {
	stats := new([]*VolumeInfo)
	for k, v := range s.volumes {
		s := new(VolumeInfo)
		s.Id, s.Size = k, v.Size()
		*stats = append(*stats, s)
	}
	bytes, _ := json.Marshal(stats)
	values := make(url.Values)
	values.Add("port", strconv.Itoa(s.Port))
	values.Add("publicServer", s.PublicServer)
	values.Add("volumes", string(bytes))
	log.Println("Registering exiting volumes", string(bytes))
	values.Add("capacity", strconv.Itoa(s.capacity))
	retString := post("http://"+mserver+"/dir/join", values)
	if retString != nil {
		newVids := new([]int)
		log.Println("Instructed to create volume",string(retString))
		e := json.Unmarshal(retString, newVids)
		if e == nil {
			for _, vid := range *newVids {
				s.volumes[uint64(vid)] = NewVolume(s.dir, uint64(vid))
				log.Println("Adding volume", vid)
			}
		}
	}
}
func (s *Store) Close() {
	for _, v := range s.volumes {
		v.Close()
	}
}
func (s *Store) Write(i uint64, n *Needle) {
	s.volumes[i].write(n)
}
func (s *Store) Read(i uint64, n *Needle) {
	s.volumes[i].read(n)
}