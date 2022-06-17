package main

import (
	"fmt"
	"os"
	"flag"
	"time"
	"sort"
	"io/ioutil"
	"strings"
	"unicode"
	"path/filepath"
)

type PatternMatcher struct {
        Pattern        string
        MatcherType    int
        CaseSensitive  bool
}

type MatcherInput struct {
        InputString     string
        OriginalIndex   int
        BestScore       int
        SecondBest      int
}

type MatcherInputSlice  []*MatcherInput

const (
        PenaltyGoDown    int16  = -2            //penalty for the insertion
        PenaltyGoRight   int16  = -2            //penalty for the deletion
        PenaltyGoCross   int16  = -4            //penalty for the substitution, which is essentialy deletion+insertion

        BonusMatch       int16  =  5          //character match
        BonusConsequtive int16  =  2          //to favor consequtive matches
        BonusPosition    int16  =  2          //to favor position like start of directory name
)

const (
        MatcherTypeFile         = 0x0001
        MatcherTypeIdentifier   = 0x0002
)

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
                matcher := InitPatternMatcher(pattern, MatcherTypeFile)
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
        fmt.Printf("%v microseconds elapsed\n", elapsed.Microseconds())

        start = time.Now()
        RankRecords(curated, matcher)
        elapsed = time.Since(start)

        fmt.Printf("%d records ranked\n", len (curated))
        fmt.Printf("%v microseconds elapsed\n", elapsed.Microseconds())

        sort.Sort(sort.Reverse(curated))

        cutoff := 20
        if cutoff > len (curated) {
                cutoff = len (curated)
        }
        for _, s := range (curated[:cutoff]) {
                fmt.Printf("%d:%d\t%s\n", s.BestScore, s.SecondBest, list[s.OriginalIndex])
        }
        return nil
}

func CurateRecords(list []string, matcher *PatternMatcher) MatcherInputSlice {
        var result MatcherInputSlice
        for i, r := range (list) {
                if !matcher.CaseSensitive {
                        r = strings.ToLower(r)
                }
                Start, End := matcher.GetFirstAndLastSymbol()
                StartIndex := strings.IndexByte(r, Start)
                EndIndex   := strings.LastIndexByte(r, End)
                Gap        := EndIndex - StartIndex + 1
                if (StartIndex == -1) || (EndIndex == - 1) || (Gap < len (matcher.Pattern))  {
                        continue
                }

                //We expand start of the input string to include preceding character
                //in case it was special character, for which we have bonus check
                r = r[StartIndex - 1 : EndIndex + 1]

                if matcher.Contains(r) {
                        mi := &MatcherInput{ InputString : r, OriginalIndex : i }
                        result = append(result, mi)
                }
        }
        return result
}

func RankRecords(list MatcherInputSlice, matcher *PatternMatcher) {
        for _, r := range (list) {
                matcher.Rank(r) 
        }
}

func InitPatternMatcher(pattern string, matcher_type int) *PatternMatcher {
        r := &PatternMatcher{ Pattern : pattern, MatcherType : matcher_type}

        r.CaseSensitive = false
        for _, s := range (pattern) {
                if unicode.IsUpper(s) {
                        r.CaseSensitive = true
                        break
                }
        }
        return r
}

func (pm *PatternMatcher) Contains(s string) bool {
        for _, c := range (pm.Pattern) {
                idx := strings.IndexByte(s, byte(c))
                if idx == -1 {
                        return false
                }
                s = s[idx + 1:]
        }
        return true
}

func (pm *PatternMatcher) Rank(mi *MatcherInput) {
        best, second_best := int16(0), int16(0)
        lp := len (pm.Pattern) + 1
        li := len (mi.InputString) + 1

        ScoreMatrix := make([]int16, lp * li)

        for y := 1; y < lp; y += 1 {
                for x := 1; x < li; x += 1 {
                        //current pattern symbol (0-indexed)
                        cp := pm.Pattern[y - 1]
                        //current input string symbol (0-indexed)
                        ci := mi.InputString[x - 1]

                        idx := y * li + x               //current score matrix cell index
                        t_idx := (y - 1) * li + x       //top neighbour cell index

                        left_neighbor := ScoreMatrix[idx - 1]
                        top_neighbor  := ScoreMatrix[t_idx]
                        top_left_nbr  := ScoreMatrix[t_idx - 1]

                        score := int16(0)

                        if cp == ci {
                                score = top_left_nbr + BonusMatch

                                //Check if symbol is special
                                if pm.IsSpecial(ci) {
                                        score += BonusPosition
                                }
                                //Check if previous symbol (if any ) was special
                                if x > 1 &&  pm.IsSpecial(mi.InputString[x - 2]) {
                                        score += BonusPosition
                                }
                                //Check if previous pair of symbols match
                                if top_left_nbr >= BonusMatch {
                                        score += BonusConsequtive
                                }
                                //Update watermarks
                                if score > best {
                                        second_best = best
                                        best        = score
                                } else if score > second_best {
                                        second_best = score
                                }
                        } else {
                                go_down  := top_neighbor + PenaltyGoDown
                                go_right := left_neighbor + PenaltyGoRight
                                go_cross := top_left_nbr + PenaltyGoCross

                                //Pick up biggest value, but make sure it is > 0
                                if score < go_down {
                                        score = go_down
                                }
                                if score < go_right {
                                        score = go_right
                                }
                                if score < go_cross {
                                        score = go_cross
                                }
                        }
                        ScoreMatrix[idx] = score
                }
        }
        mi.BestScore  = int(best)
        mi.SecondBest = int(second_best)
}

func (pm *PatternMatcher) GetFirstAndLastSymbol() (byte, byte) {
        return byte(pm.Pattern[0]), byte(pm.Pattern[len (pm.Pattern) - 1])
}

func (pm *PatternMatcher) IsSpecial(c byte) bool {
        if c == '/' || c == '\\' {
                return true
        }
        return false
}

func (mis MatcherInputSlice) Len() int {
        return len (mis)
}

func (mis MatcherInputSlice) Swap(i, j int) {
        mis[i], mis[j] = mis[j], mis[i]
}

func (mis MatcherInputSlice) Less(i, j int) bool {
        if mis[i].BestScore < mis[j].BestScore {
                return true
        } else if mis[i].BestScore == mis[j].BestScore {
                if mis[i].SecondBest < mis[j].SecondBest {
                        return true
                } else {
                        return false
                }
        } else {
                return false
        }
}

