package files

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/smira/aptly/utils"

	. "gopkg.in/check.v1"
)

type PublishedStorageSuite struct {
	root    string
	storage *PublishedStorage
}

var _ = Suite(&PublishedStorageSuite{})

func (s *PublishedStorageSuite) SetUpTest(c *C) {
	s.root = c.MkDir()
	s.storage = NewPublishedStorage(s.root)
}

func (s *PublishedStorageSuite) TestPublicPath(c *C) {
	c.Assert(s.storage.PublicPath(), Equals, filepath.Join(s.root, "public"))
}

func (s *PublishedStorageSuite) TestMkDir(c *C) {
	err := s.storage.MkDir("ppa/dists/squeeze/")
	c.Assert(err, IsNil)

	_, err = os.Stat(filepath.Join(s.storage.rootPath, "ppa/dists/squeeze/"))
	c.Assert(err, IsNil)
}

func (s *PublishedStorageSuite) TesPutFile(c *C) {
	err := s.storage.MkDir("ppa/dists/squeeze/")
	c.Assert(err, IsNil)

	err = s.storage.PutFile("ppa/dists/squeeze/Release", "/dev/null")
	c.Assert(err, IsNil)

	_, err = os.Stat(filepath.Join(s.storage.rootPath, "ppa/dists/squeeze/Release"))
	c.Assert(err, IsNil)
}

func (s *PublishedStorageSuite) TestFilelist(c *C) {
	err := s.storage.MkDir("ppa/pool/main/a/ab/")
	c.Assert(err, IsNil)

	err = s.storage.PutFile("ppa/pool/main/a/ab/a.deb", "/dev/null")
	c.Assert(err, IsNil)

	err = s.storage.PutFile("ppa/pool/main/a/ab/b.deb", "/dev/null")
	c.Assert(err, IsNil)

	list, err := s.storage.Filelist("ppa/pool/main/")
	c.Check(err, IsNil)
	c.Check(list, DeepEquals, []string{"a/ab/a.deb", "a/ab/b.deb"})

	list, err = s.storage.Filelist("ppa/pool/doenstexist/")
	c.Check(err, IsNil)
	c.Check(list, DeepEquals, []string{})
}

func (s *PublishedStorageSuite) TestRenameFile(c *C) {
	err := s.storage.MkDir("ppa/dists/squeeze/")
	c.Assert(err, IsNil)

	err = s.storage.PutFile("ppa/dists/squeeze/Release", "/dev/null")
	c.Assert(err, IsNil)

	err = s.storage.RenameFile("ppa/dists/squeeze/Release", "ppa/dists/squeeze/InRelease")
	c.Check(err, IsNil)

	_, err = os.Stat(filepath.Join(s.storage.rootPath, "ppa/dists/squeeze/InRelease"))
	c.Assert(err, IsNil)
}

func (s *PublishedStorageSuite) TestRemoveDirs(c *C) {
	err := s.storage.MkDir("ppa/dists/squeeze/")
	c.Assert(err, IsNil)

	err = s.storage.PutFile("ppa/dists/squeeze/Release", "/dev/null")
	c.Assert(err, IsNil)

	err = s.storage.RemoveDirs("ppa/dists/", nil)
	c.Assert(err, IsNil)

	_, err = os.Stat(filepath.Join(s.storage.rootPath, "ppa/dists/squeeze/Release"))
	c.Assert(err, NotNil)
	c.Assert(os.IsNotExist(err), Equals, true)
}

func (s *PublishedStorageSuite) TestRemove(c *C) {
	err := s.storage.MkDir("ppa/dists/squeeze/")
	c.Assert(err, IsNil)

	err = s.storage.PutFile("ppa/dists/squeeze/Release", "/dev/null")
	c.Assert(err, IsNil)

	err = s.storage.Remove("ppa/dists/squeeze/Release")
	c.Assert(err, IsNil)

	_, err = os.Stat(filepath.Join(s.storage.rootPath, "ppa/dists/squeeze/Release"))
	c.Assert(err, NotNil)
	c.Assert(os.IsNotExist(err), Equals, true)
}

func (s *PublishedStorageSuite) TestLinkFromPool(c *C) {
	tests := []struct {
		prefix           string
		component        string
		sourcePath       string
		poolDirectory    string
		expectedFilename string
	}{
		{ // package name regular
			prefix:           "",
			component:        "main",
			sourcePath:       "mars-invaders_1.03.deb",
			poolDirectory:    "m/mars-invaders",
			expectedFilename: "pool/main/m/mars-invaders/mars-invaders_1.03.deb",
		},
		{ // lib-like filename
			prefix:           "",
			component:        "main",
			sourcePath:       "libmars-invaders_1.03.deb",
			poolDirectory:    "libm/libmars-invaders",
			expectedFilename: "pool/main/libm/libmars-invaders/libmars-invaders_1.03.deb",
		},
		{ // duplicate link, shouldn't panic
			prefix:           "",
			component:        "main",
			sourcePath:       "mars-invaders_1.03.deb",
			poolDirectory:    "m/mars-invaders",
			expectedFilename: "pool/main/m/mars-invaders/mars-invaders_1.03.deb",
		},
		{ // prefix & component
			prefix:           "ppa",
			component:        "contrib",
			sourcePath:       "libmars-invaders_1.04.deb",
			poolDirectory:    "libm/libmars-invaders",
			expectedFilename: "pool/contrib/libm/libmars-invaders/libmars-invaders_1.04.deb",
		},
	}

	pool := NewPackagePool(s.root)

	for _, t := range tests {
		tmpPath := filepath.Join(c.MkDir(), t.sourcePath)
		err := ioutil.WriteFile(tmpPath, []byte("Contents"), 0644)
		c.Assert(err, IsNil)

		srcPoolPath, err := pool.Import(tmpPath, t.sourcePath, &utils.ChecksumInfo{MD5: "c1df1da7a1ce305a3b60af9d5733ac1d"}, false)
		c.Assert(err, IsNil)

		err = s.storage.LinkFromPool(filepath.Join(t.prefix, "pool", t.component, t.poolDirectory), pool, srcPoolPath, utils.ChecksumInfo{}, false)
		c.Assert(err, IsNil)

		st, err := os.Stat(filepath.Join(s.storage.rootPath, t.prefix, t.expectedFilename))
		c.Assert(err, IsNil)

		info := st.Sys().(*syscall.Stat_t)
		c.Check(int(info.Nlink), Equals, 3)
	}

	// test linking files to duplicate final name
	tmpPath := filepath.Join(c.MkDir(), "mars-invaders_1.03.deb")
	err := ioutil.WriteFile(tmpPath, []byte("Contents"), 0644)
	c.Assert(err, IsNil)

	srcPoolPath, err := pool.Import(tmpPath, "mars-invaders_1.03.deb", &utils.ChecksumInfo{MD5: "02bcda7a1ce305a3b60af9d5733ac1d"}, true)
	c.Assert(err, IsNil)

	st, err := pool.Stat(srcPoolPath)
	c.Assert(err, IsNil)
	nlinks := int(st.Sys().(*syscall.Stat_t).Nlink)

	err = s.storage.LinkFromPool(filepath.Join("", "pool", "main", "m/mars-invaders"), pool, srcPoolPath, utils.ChecksumInfo{}, false)
	c.Check(err, ErrorMatches, ".*file already exists and is different")

	st, err = pool.Stat(srcPoolPath)
	c.Assert(err, IsNil)
	c.Check(int(st.Sys().(*syscall.Stat_t).Nlink), Equals, nlinks)

	// linking with force
	err = s.storage.LinkFromPool(filepath.Join("", "pool", "main", "m/mars-invaders"), pool, srcPoolPath, utils.ChecksumInfo{}, true)
	c.Check(err, IsNil)

	st, err = pool.Stat(srcPoolPath)
	c.Assert(err, IsNil)
	c.Check(int(st.Sys().(*syscall.Stat_t).Nlink), Equals, nlinks+1)
}
