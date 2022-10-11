package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"
)

type Solution struct {
	Answer  string
	Path    string
	Problem int
}

func SourceSolutions(ctx context.Context, root string) <-chan Solution {
	solutions := make(chan Solution)

	go func() {
		defer close(solutions)

		files, err := os.ReadDir(root)
		if err != nil {
			log.Fatalf("failed to list directory contents: %v", err)
		}

		for _, file := range files {
			if path.Ext(file.Name()) != ".go" || file.Name() == "validate.go" {
				continue
			}

			problemNumber, _ := strconv.Atoi(file.Name()[:len(file.Name())-len(path.Ext(file.Name()))])

			select {
			case solutions <- Solution{Path: file.Name(), Problem: problemNumber}:
			case <-ctx.Done():
				return
			}
		}
	}()

	return solutions
}

func RunSolutions(ctx context.Context, unprocessed <-chan Solution) <-chan Solution {
	numWorkers := runtime.GOMAXPROCS(0)
	var wg sync.WaitGroup
	wg.Add(numWorkers)

	solutions := make(chan Solution)

	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()

			for solution := range unprocessed {
				answer, err := exec.Command("go", "run", solution.Path).CombinedOutput()
				if err != nil {
					log.Fatalf("failed to run solution %d: %v", solution.Problem, err)
				}

				solution.Answer = string(bytes.TrimSuffix(answer, []byte{'\n'}))

				select {
				case solutions <- solution:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		defer close(solutions)
		wg.Wait()
	}()

	return solutions
}

func FetchAnswers() []string {
	file, err := os.Open("answers.txt")
	if err != nil {
		log.Fatalf("failed to open 'answer.txt': %v", err)
	}
	defer file.Close()

	var (
		answers []string
		scanner = bufio.NewScanner(file)
	)

	for scanner.Scan() {
		answers = append(answers, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("failed to read answers: %v", err)
	}

	return answers
}

func main() {
	start := time.Now()

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	var (
		solutions []Solution
		answers   = FetchAnswers()
	)

	for solution := range RunSolutions(ctx, SourceSolutions(ctx, ".")) {
		solutions = append(solutions, solution)
	}

	sort.Slice(solutions, func(i, j int) bool { return solutions[i].Problem < solutions[j].Problem })

	for _, solution := range solutions {
		var (
			correctness = "correct"
			equality    = "=="
		)

		if solution.Answer != answers[solution.Problem-1] {
			correctness = "incorrect"
			equality = "!="
		}

		fmt.Printf("Problem %03d: Output is %s '%s %s %s'\n", solution.Problem, correctness, solution.Answer, equality,
			answers[solution.Problem-1])
	}

	fmt.Printf("Completed in: %v\n", time.Since(start))
}
