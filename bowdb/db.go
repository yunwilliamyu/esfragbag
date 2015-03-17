package bowdb

import (
	"archive/tar"
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	path "path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yunwilliamyu/esfragbag"
	"github.com/yunwilliamyu/esfragbag/bow"
)

const (
	fileBowDB   = "bow.db"
	fileFragLib = "frag-lib.json"
)

// DB represents a BOW database. It is always connected to a particular
// fragment library. In particular, the disk representation of the database is
// a directory with a copy of the fragment library used to create the database
// and a binary formatted file of all the frequency vectors computed.
type DB struct {
	// The fragment library used to make this database.
	Lib fragbag.Library

	// The name of this database (which always corresponds to the base
	// name of the database's file path).
	Name string

	// The set of entries read from disk when reading a bow DB.
	// This is populated by ReadAll.
	Entries     []bow.Bowed
	readAllLock *sync.Mutex // Protects concurrent calls of ReadAll

	fileBuf *bufio.Reader // A buffer for reading the bow db.

	entryBuf []byte    // Temporary buffer for reading DB entries.
	bowPool  []float32 // Memory pool for fragment frequencies.
	bowLast  int       // Last index used in bow pool.
	dataPool []byte    // Memory pool for entry data.
	dataLast int       // Last index used in data pool.

	tw          *tar.Writer    // The writer archive.
	saveBuf     *bytes.Buffer  // Buffer for bowdb while writing.
	writeBuf    *bytes.Buffer  // Temporary buffer for binary.
	writingDone chan struct{}  // Indicate when writing is done.
	entryChan   chan bow.Bowed // Concurrent writing.

    file *os.File // Save the file that was opened
}

// Open opens a new BOW database for reading. In particular, all entries
// in the database will be loaded into memory.
func Open(fpath string) (*DB, error) {
	var err error

	db := &DB{
		Name:        path.Base(fpath),
		readAllLock: new(sync.Mutex),
	}

	dbf, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(dbf)

	if _, err := tr.Next(); err != nil { // the dir header, skip it
		return nil, err
	}
	if _, err := tr.Next(); err != nil { // the flib header
		return nil, err
	}

	db.Lib, err = fragbag.Open(tr)
	if err != nil {
		return nil, err
	}

	if _, err := tr.Next(); err != nil { // the bow db header
		return nil, err
	}
	db.fileBuf = bufio.NewReaderSize(tr, 1<<20)
    db.file = dbf
	return db, nil
}

// ReadAll reads all entries from disk and returns them in a slice.
// Subsequent calls do not read from disk; the already read entries are
// returned.
//
// ReadAll will panic if it is called on a database that was made with the
// Create function.
func (db *DB) ReadAll() ([]bow.Bowed, error) {
	if db.readAllLock == nil {
		panic("DB.ReadAll cannot be called when the database is being written")
	}

	db.readAllLock.Lock()
	defer db.readAllLock.Unlock()

	if db.Entries != nil {
		return db.Entries, nil
	}
	db.Entries = make([]bow.Bowed, 0, 10000)
	for {
		entry, err := db.read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		db.Entries = append(db.Entries, *entry)
	}
	return db.Entries, nil
}

// Create creates a new BOW database on disk at 'dir'. If the directory
// already exists or cannot be created, an error is returned.
//
// When you're finished adding entries, you must call Close.
//
// Once a BOW database is created, it cannot be modified. (This restriction
// may be lifted in the future.)
func Create(lib fragbag.Library, fpath string) (*DB, error) {
	if _, err := os.Stat(fpath); err == nil || !os.IsNotExist(err) {
		return nil, fmt.Errorf("BOW database '%s' already exists.", fpath)
	}
	outf, err := os.Create(fpath)
	if err != nil {
		return nil, err
	}

	db := &DB{
		Lib:  lib,
		Name: path.Base(fpath),

		tw:          tar.NewWriter(outf),
		saveBuf:     new(bytes.Buffer),
		writeBuf:    new(bytes.Buffer),
		entryChan:   make(chan bow.Bowed),
		writingDone: make(chan struct{}),
	}

	// Put all bow DB files in a directory within the archive.
	hdrDir := db.newHdrDir(db.dirName())
	if err := db.tw.WriteHeader(hdrDir); err != nil {
		return nil, err
	}

	// Create an entry for the fragment library. Copy the bytes.
	flibBytes := new(bytes.Buffer)
	if err := fragbag.Save(flibBytes, db.Lib); err != nil {
		return nil, fmt.Errorf("Could not copy fragment library: %s", err)
	}
	hdr := db.newHdr(fileFragLib, flibBytes.Len())
	if err := db.tw.WriteHeader(hdr); err != nil {
		return nil, err
	}
	if _, err := db.tw.Write(flibBytes.Bytes()); err != nil {
		return nil, err
	}

	// Now spin up a goroutine that is responsible for writing entries.
	go func() {
		for entry := range db.entryChan {
			if err = db.write(entry); err != nil {
				log.Printf("Could not write to %s: %s", fileBowDB, err)
			}
		}
		db.writingDone <- struct{}{}
	}()
    db.file = outf
	return db, nil
}

// Add will add a row to the database. It is safe to call `Add` from multiple
// goroutines. The bowed value given must have been computed with the fragment
// library given to Create.
//
// Add will panic if it is called on a BOW database that has been opened for
// reading.
func (db *DB) Add(e bow.Bowed) {
	if db.entryChan == nil {
		panic("Cannot add to a BOW database opened in read mode.")
	}
	db.entryChan <- e
}

// Close should be called when done reading/writing a BOW db.
func (db *DB) Close() error {
	if db.tw != nil {
		close(db.entryChan)
		<-db.writingDone

		hdr := db.newHdr(fileBowDB, db.saveBuf.Len())
		if err := db.tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("Could not write TAR header for bow db: %s", err)
		}
		if _, err := db.tw.Write(db.saveBuf.Bytes()); err != nil {
			return fmt.Errorf("Could not write contents of bow db: %s", err)
		}

		if err := db.tw.Close(); err != nil {
			return fmt.Errorf("Could not close bowdb archive: %s", err)
		}
	}
	//return nil
	return db.file.Close()
}

// String returns the name of the database.
func (db *DB) String() string {
	return db.Name
}

func (db *DB) newHdr(name string, size int) *tar.Header {
	now := time.Now()
	return &tar.Header{
		Name:       path.Join(db.dirName(), name),
		Mode:       0644,
		Uid:        os.Getuid(),
		Gid:        os.Getgid(),
		Size:       int64(size),
		ModTime:    now,
		AccessTime: now,
		ChangeTime: now,
	}
}

func (db *DB) newHdrDir(name string) *tar.Header {
	h := db.newHdr(name, 0)
	h.Name = name
	h.Typeflag = tar.TypeDir
	h.Mode = 0755
	return h
}

func (db *DB) dirName() string {
	return strings.TrimSuffix(db.Name, path.Ext(db.Name))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// read will read a single entry from the BOW database.
//
// It would be much nicer to use the binary package here (like we do for
// reading), but we need to be as fast here as possible. (It looks like
// there is a fair bit of allocation going on in the binary package.)
// Benchmarks are gone in the wind...
func (db *DB) read() (*bow.Bowed, error) {

	// Read in the id string.
	if err := db.readItem(); err != nil {
		return nil, err
	}
	id := string(db.entryBuf)

	// Read in the arbitrary data.
	if err := db.readItem(); err != nil {
		return nil, err
	}
	data := db.newData(len(db.entryBuf))
	copy(data, db.entryBuf)

	// Now read in the BOW.
	if err := db.readItem(); err != nil {
		return nil, err
	}

	freqs := db.newBow()
	// Advance 6 bytes at a time. 2 bytes for the fragment index and
	// 4 bytes for the fragment frequency.
	for i := 0; i < len(db.entryBuf); i += 6 {
		fragi := binary.BigEndian.Uint16(db.entryBuf[i : i+2])
		freqs[fragi] = math.Float32frombits(
			binary.BigEndian.Uint32(db.entryBuf[i+2 : i+6]))
	}
	return &bow.Bowed{Id: id, Data: data, Bow: bow.Bow{freqs}}, nil
}

func (db *DB) newBow() []float32 {
	libSize := db.Lib.Size()
	if db.bowLast+libSize >= cap(db.bowPool) {
		db.bowPool = make([]float32, libSize*10000)
		db.bowLast = 0
	}
	b := db.bowPool[db.bowLast : db.bowLast+libSize]
	db.bowLast += libSize
	return b
}

func (db *DB) newData(size int) []byte {
	if size == 0 {
		return nil
	}
	if db.dataLast+size >= cap(db.dataPool) {
		db.dataPool = make([]byte, 5*(1<<20))
		db.dataLast = 0
	}
	d := db.dataPool[db.dataLast : db.dataLast+size]
	db.dataLast += size
	return d
}

func (db *DB) readItem() error {
	if err := db.readNBytes(4); err != nil {
		return err
	}
	itemLen := binary.BigEndian.Uint32(db.entryBuf)

	if err := db.readNBytes(int(itemLen)); err != nil {
		return err
	}
	return nil
}

func (db *DB) readNBytes(n int) error {
	if db.entryBuf == nil || n > cap(db.entryBuf) {
		db.entryBuf = make([]byte, n)
	}
	db.entryBuf = db.entryBuf[0:n]

	nread := 0
	for nread < n {
		if thisn, err := db.fileBuf.Read(db.entryBuf[nread:]); err != nil {
			if err == io.EOF {
				return io.EOF
			}
			return fmt.Errorf("Error reading item: %s", err)
		} else if thisn == 0 {
			return fmt.Errorf("Expected item with length %d, but got %d",
				n, nread)
		} else {
			nread += thisn
		}
	}
	return nil
}

func (db *DB) write(entry bow.Bowed) error {
	libSize := db.Lib.Size()

	db.writeBuf.WriteString(entry.Id)
	if err := db.writeItem(); err != nil {
		return err
	}

	db.writeBuf.Write(entry.Data)
	if err := db.writeItem(); err != nil {
		return err
	}

	// Store BOWs as sparse frequency vectors.
	for i := 0; i < libSize; i++ {
		f := entry.Bow.Freqs[i]
		if f > 0 {
			if err := binw(db.writeBuf, uint16(i)); err != nil {
				return fmt.Errorf("Error writing bow '%s': %s", entry.Id, err)
			}
			if err := binw(db.writeBuf, f); err != nil {
				return fmt.Errorf("Error writing BOW '%s': %s", entry.Id, err)
			}
		}
	}
	if err := db.writeItem(); err != nil {
		return err
	}
	return nil
}

func (db *DB) writeItem() error {
	itemLen := uint32(db.writeBuf.Len())
	if err := binw(db.saveBuf, itemLen); err != nil {
		return fmt.Errorf("Could not write item size: %s", err)
	}
	if _, err := db.saveBuf.Write(db.writeBuf.Bytes()); err != nil {
		return fmt.Errorf("Could not write item: %s", err)
	}
	db.writeBuf.Reset()
	return nil
}

func binw(w io.Writer, v interface{}) error {
	return binary.Write(w, binary.BigEndian, v)
}
