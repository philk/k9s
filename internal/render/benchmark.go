package render

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/derailed/tview"
	"github.com/gdamore/tcell"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	totalRx = regexp.MustCompile(`Total:\s+([0-9.]+)\ssecs`)
	reqRx   = regexp.MustCompile(`Requests/sec:\s+([0-9.]+)`)
	okRx    = regexp.MustCompile(`\[2\d{2}\]\s+(\d+)\s+responses`)
	errRx   = regexp.MustCompile(`\[[4-5]\d{2}\]\s+(\d+)\s+responses`)
	toastRx = regexp.MustCompile(`Error distribution`)
)

// Benchmark renders a benchmarks to screen.
type Benchmark struct{}

// ColorerFunc colors a resource row.
func (Benchmark) ColorerFunc() ColorerFunc {
	return func(ns string, re RowEvent) tcell.Color {
		c := tcell.ColorPaleGreen
		statusCol := 2
		if strings.TrimSpace(re.Row.Fields[statusCol]) != "pass" {
			c = ErrColor
		}
		return c
	}
}

// Header returns a header row.
func (Benchmark) Header(ns string) HeaderRow {
	return HeaderRow{
		Header{Name: "NAMESPACE"},
		Header{Name: "NAME"},
		Header{Name: "STATUS"},
		Header{Name: "TIME"},
		Header{Name: "REQ/S", Align: tview.AlignRight},
		Header{Name: "2XX", Align: tview.AlignRight},
		Header{Name: "4XX/5XX", Align: tview.AlignRight},
		Header{Name: "REPORT"},
		Header{Name: "AGE", Decorator: AgeDecorator},
	}
}

// Render renders a K8s resource to screen.
func (b Benchmark) Render(o interface{}, ns string, r *Row) error {
	bench, ok := o.(BenchInfo)
	if !ok {
		return fmt.Errorf("expecting benchinfo but got `%T", o)
	}

	data, err := b.readFile(bench.Path)
	if err != nil {
		return fmt.Errorf("Unable to load bench file %s", bench.Path)
	}

	r.ID = bench.Path
	r.Fields = make(Fields, len(b.Header(ns)))
	if err := b.initRow(r.Fields, bench.File); err != nil {
		return err
	}
	b.augmentRow(r.Fields, data)

	return nil
}

// ----------------------------------------------------------------------------
// Helpers...

func (Benchmark) readFile(file string) (string, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (Benchmark) initRow(row Fields, f os.FileInfo) error {
	tokens := strings.Split(f.Name(), "_")
	if len(tokens) < 2 {
		return fmt.Errorf("Invalid file name %s", f.Name())
	}
	row[0] = tokens[0]
	row[1] = tokens[1]
	row[7] = f.Name()
	row[8] = timeToAge(f.ModTime())

	return nil
}

func (b Benchmark) augmentRow(fields Fields, data string) {
	if len(data) == 0 {
		return
	}

	col := 2
	fields[col] = "pass"
	mf := toastRx.FindAllStringSubmatch(data, 1)
	if len(mf) > 0 {
		fields[col] = "fail"
	}
	col++

	mt := totalRx.FindAllStringSubmatch(data, 1)
	if len(mt) > 0 {
		fields[col] = mt[0][1]
	}
	col++

	mr := reqRx.FindAllStringSubmatch(data, 1)
	if len(mr) > 0 {
		fields[col] = mr[0][1]
	}
	col++

	ms := okRx.FindAllStringSubmatch(data, -1)
	fields[col] = b.countReq(ms)
	col++

	me := errRx.FindAllStringSubmatch(data, -1)
	fields[col] = b.countReq(me)
}

func (Benchmark) countReq(rr [][]string) string {
	if len(rr) == 0 {
		return "0"
	}

	var sum int
	for _, m := range rr {
		if m, err := strconv.Atoi(string(m[1])); err == nil {
			sum += m
		}
	}
	return asNum(sum)
}

// AsNumb prints a number with thousand separator.
func asNum(n int) string {
	p := message.NewPrinter(language.English)
	return p.Sprintf("%d", n)
}

// BenchInfo represents benchmark run info.
type BenchInfo struct {
	File os.FileInfo
	Path string
}

// GetObjectKind returns a schema object.
func (BenchInfo) GetObjectKind() schema.ObjectKind {
	return nil
}

// DeepCopyObject returns a container copy.
func (b BenchInfo) DeepCopyObject() runtime.Object {
	return b
}