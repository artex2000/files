package main

import (
	"fmt"
	"os"
	"flag"
	"time"
	"io/ioutil"
	"strings"
	"unicode"
	"path/filepath"
)

type PatternMatcher struct {
        Pattern        string
        CaseSensitive  bool
}

//Command line flags usage as follows:
//Scan mode
//      Set filter (file extension) - optional
//      Set output file name - optional
//      Provide target directory
//
//Rank mode
//      Provide matching pattern
//
var (
        filter   string
        output   string
        pattern  string
        scan     bool
        rank     bool
)

func init() {
        flag.StringVar(&output, "o", "scan_result.txt", "output filename")
        flag.StringVar(&filter, "f", "", "file type to collect")
        flag.StringVar(&pattern, "p", "", "pattern to rank (at least 3 symbols)")
        flag.BoolVar(&scan, "s", false, "scanning mode")
        flag.BoolVar(&rank, "r", false, "ranking mode")
}

func Usage() {
        fmt.Printf("Usage:\n\tfiles.exe -s [-f ext] [-o filename] directory\n")
        fmt.Printf("\tfiles.exe -r -p pattern inputfile\n")
        flag.PrintDefaults()
}

func main() {
        flag.Parse()
        if scan {
                if rank {
                        fmt.Println("files.exe: Can't use -r and -s options together")
                        os.Exit(1)
                } else if flag.NArg() == 0 {
                        fmt.Println("files.exe: Target directory not specified")
                        os.Exit(1)
                }
        } else if rank {
                if scan {
                        fmt.Println("files.exe: Can't use -r and -s options together")
                        os.Exit(1)
                } else if flag.NArg() == 0 {
                        fmt.Println("files.exe: Input filename not specified")
                        os.Exit(1)
                } else if len (pattern) < 3 {
                        fmt.Println("files.exe: Matching pattern is too short or empty")
                        os.Exit(1)
                }
        } else {
                fmt.Println("files.exe: One of the options [-r | -s] must be specified")
                os.Exit(1)
        }

        input := flag.Arg(0)
        if scan {
                err := ScanDirectory(input)
                if err != nil {
                        fmt.Println("Directory scan is failed")
                        os.Exit(1)
                }
        } else {
                matcher := InitPatternMatcher(pattern)
                err := MatchPattern(input, matcher)
                if err != nil {
                        fmt.Println("Pattern match is failed")
                        os.Exit(1)
                }
        }
}

func ScanDirectory(top string) error {
        var dirs  []string
        var outs  []string

        //Get root directory
	root, err := filepath.Abs(top)
        if err != nil {
                fmt.Printf("Cannot get absolute path for %v - %v\n", top, err)
                return err
        }

        //Add dot to the extension to filter to be in line with filepath.Ext return values
	if len (filter) > 0 {
		filter = "." + filter
	}

        //Read root directory
        start := time.Now()

        files, err := ReadDirectory(root)
        if err != nil {
                fmt.Printf("Cannot read directory %v - %v\n", root, err)
                return err
        }

        for _, f := range (files) {
                n := f.Name()
                if f.IsDir() {
                        n = filepath.Join(root, n)
                        dirs = append (dirs, n)
                } else if filter == filepath.Ext(n) {
                        n = filepath.Join(root, n)
                        outs = append (outs, n)
                }
        }

        //Now do the same for subdirectories
        dir_count := 1          //we already processed root
        for {
                var next string
                if l := len (dirs); l > 0 {
                        next = dirs[0]
                        dirs = dirs[1:]
                        dir_count += 1
                } else {
                        break;
                }

                files, err = ReadDirectory(next)
                if err != nil {
                        continue
                }

                for _, f := range (files) {
                        n := f.Name()
                        if f.IsDir() {
                                n = filepath.Join(next, n)
                                dirs = append (dirs, n)
                        } else if filter == filepath.Ext(n) {
                                n = filepath.Join(next, n)
                                outs = append (outs, n)
                        }
                }
        }

        elapsed := time.Since(start)

        fmt.Printf("%d directories processed\n", dir_count)
        fmt.Printf("%d files with extention %s found\n", len (outs), filter)
        fmt.Printf("%v milliseconds elapsed\n", elapsed.Milliseconds())

        //Write found files to output file
        out, err := filepath.Abs(output)
        if err != nil {
                fmt.Printf("Cannot get absolute path for %v - %v\n", output, err)
                return err
        }

        f, err := os.Create(out)
        if err != nil {
                fmt.Printf("Cannot create file %v - %v\n", out, err)
                return err
        }
        defer f.Close()

        _, err = f.WriteString(strings.Join(outs, "\r\n"))
        if err != nil {
                fmt.Printf("Error writing file %v - %v\n", out, err)
                return err
        }
        return nil
}
        
func ReadDirectory(FullPath string) ([]os.FileInfo, error) {
        f, err := os.Open(FullPath)
        if err != nil {
                return nil, err
        }
        defer f.Close()

        files, err := f.Readdir(-1)
        if err != nil {
                return nil, err
        }
        return files, nil 
}

func MatchPattern(name string, matcher *PatternMatcher) error {
        //Get list of files to rank against pattern
	file, err := filepath.Abs(name)
        if err != nil {
                fmt.Printf("Cannot get absolute path for %v - %v\n", name, err)
                return err
        }

        data, err := ioutil.ReadFile(file)
        if err != nil {
                fmt.Printf("Cannot read file %v - %v\n", file, err)
                return err
        }

        list := strings.Split(string(data), "\r\n")
        fmt.Printf("%d records loaded\n", len (list))

        start := time.Now()
        curated := CurateRecords(list, matcher)
        elapsed := time.Since(start)

        fmt.Printf("%d records curated to %d\n", len (list), len (curated))
        fmt.Printf("%v milliseconds elapsed\n", elapsed.Milliseconds())
        return nil
}

func CurateRecords(list []string, matcher *PatternMatcher) []string {
        var result []string
        for _, r := range (list) {
                if !matcher.CaseSensitive {
                        r = strings.ToLower(r)
                }
                if index, ok := matcher.Contains(r); ok {
                        result = append(result, r[index:])
                }
        }
        return result
}

func RankRecords(list []string, matcher *PatternMatcher) {
        ranks := make([]int, len (list))
        for i, r := range (list) {
                ranks[i] = matcher.Rank(r) 
        }
}

func InitPatternMatcher(pattern string) *PatternMatcher {
        r := &PatternMatcher{}
        r.Pattern = pattern
        r.CaseSensitive = false
        for _, s := range (pattern) {
                if unicode.IsUpper(s) {
                        r.CaseSensitive = true
                        break
                }
        }
        return r
}

func (pm *PatternMatcher) Contains(s string) (int, bool) {
        PrefixSkipped := false
        StartIndex    := 0
        for _, c := range (pm.Pattern) {
                idx := strings.IndexByte(s, byte(c))
                if idx == -1 {
                        return 0, false
                }

                if !PrefixSkipped {
                        s = s[idx:]
                        PrefixSkipped = true
                        StartIndex    = idx
                }
        }
        return StartIndex, true
}

func (pm *PatternMatcher) Rank(s string) int {
        return 0
}


