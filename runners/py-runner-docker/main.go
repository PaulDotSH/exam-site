package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"github.com/vmihailenco/msgpack/v5"
	"io/ioutil"
	"os"
	"os/exec"
	"time"
)

const (
	pyName      = "f.py"
	timeout      = 15
)

//TODO: if exec before or exec after is less than the Outputs length, then do the same exec before and after every time
type ProblemData struct {
	Code          string
	ExecBefore    []string
	ExecAfter     []string
	FilesToMake   [][]File //Every exercise can have multiple files, this must be an array
	ExpectedFiles [][]File
	O             []string //Expected outputs
	E             []string
	I             []string //Check if this should be a 2d array instead
	Private       []bool
}

type AnswerData struct {
	Error string
}

type File struct {
	Name    string
	Content string
}

func init() {
	if len(os.Args) < 2 {
		fmt.Println("No argument specified")
		os.Exit(0)
	}
}

//Decode pbdata from base64 encoded msgpack
func (p *ProblemData) Decode(decode string) {
	b, err := base64.StdEncoding.DecodeString(decode)
	if err != nil {
		panic(err)
	}
	err = msgpack.Unmarshal(b, &p)
	if err != nil {
		panic(err)
	}
}
func main() {
	var answer AnswerData
	//Get decode base64 to string, and parse that
	var data ProblemData
	data.Decode(os.Args[1])

	ctx, cancel := context.WithTimeout(context.Background(), timeout*time.Second)
	defer cancel()

	//We do this in case the user runs the same code each time after but not before or viceversa
	for len(data.ExecAfter) > len(data.ExecBefore) {
		data.ExecBefore = append(data.ExecBefore, data.ExecBefore[0])
	}
	for len(data.ExecAfter) < len(data.ExecBefore) {
		data.ExecAfter = append(data.ExecAfter, data.ExecAfter[0])
	}

	filesToMakeLen := len(data.FilesToMake)
	expectedFilesLen := len(data.ExpectedFiles)

	if len(data.ExecBefore) > 1 {
		//For each exercise
		for i := 0; i < len(data.I); i++ {
			writeCodeToFile(&data, i)
			if data.FilesToMake != nil && filesToMakeLen > i {
				MakeFiles(data.FilesToMake[i])
			}
			stdout, stderr, err := Run(&ctx, data.I[i])
			if !answer.IsCorrect(stdout, stderr, err, &data, i) {
				break
			}
			if data.ExpectedFiles != nil && expectedFilesLen > i && !answer.AreFilesCorrect(data.ExpectedFiles[i]) {
				break
			}
		}
		DisplayAnswer(&answer)
		return
	}

	//Do not compile each time for no reason
	writeCodeToFile(&data, 0)
	for i := 0; i < len(data.I); i++ {
		if data.FilesToMake != nil && filesToMakeLen > i {
			MakeFiles(data.FilesToMake[i])
		}
		stdout, stderr, err := Run(&ctx, data.I[i])
		if !answer.IsCorrect(stdout, stderr, err, &data, i) {
			break
		}
		if data.ExpectedFiles != nil && expectedFilesLen > i && !answer.AreFilesCorrect(data.ExpectedFiles[i]) {
			break
		}
		//fmt.Printf("Stdout:\n%vStderr:\n%vErr:\n%v\n",stdout,stderr,err)
	}
	DisplayAnswer(&answer)
}

func MakeFiles(files []File) {
	for i := 0; i < len(files); i++ {
		f, err := os.Create(files[i].Name)
		if err != nil {
			//Add this to a msg pack
			fmt.Print("Error while making the files, please contact your teacher", err, files[i].Name, files[i].Content)
			os.Exit(0)
		}
		f.Write([]byte(files[i].Content))
	}
}

func DisplayAnswer(data *AnswerData) {
	if data.Error == "" {
		fmt.Print("Ok")
		return
	}
	fmt.Print(data.Error)
}

func (a *AnswerData) IsCorrect(stdout, stderr string, err error, data *ProblemData, index int) bool {
	if err != nil {
		if data.Private[index] {
			a.Error = fmt.Sprintf("Test %v failed\n", index)
			return false
		}
		a.Error = fmt.Sprintf("Test %v failed with error %v\n", index, err)
		return false
	}

	if data.O != nil && stdout != data.O[index] {
		if data.Private[index] {
			a.Error = fmt.Sprintf("Test %v failed\n", index)
			return false
		}
		a.Error = fmt.Sprintf("Test %v\nExpected output\n%v\nActual Output\n%v\n", index, data.O[index], stdout)
		return false
	}
	if data.E != nil && stderr != data.E[index] {
		if data.Private[index] {
			a.Error = fmt.Sprintf("Test %v failed\n", index)
			return false
		}
		a.Error = fmt.Sprintf("Test %v\nExpected stderr\n%v\nActual Stderr\n%v\n", index, data.E[index], stdout)
		return false
	}
	return true
}

func (a *AnswerData) AreFilesCorrect(files []File) bool {
	for i := 0; i < len(files); i++ {
		f, err := os.OpenFile(files[i].Name, os.O_RDONLY, 0755)
		if err != nil {
			a.Error = err.Error()
			return false
		}
		defer f.Close()
		b, err := ioutil.ReadAll(f)
		if err != nil {
			a.Error = err.Error()
			return false
		}
		if files[i].Content != string(b) {
			a.Error = fmt.Sprintf("File %v does not have the correct contents\nExpected:\n%v\nGot:\n%v\n", files[i].Name, files[i].Content, string(b))
			return false
		}
	}
	return true
}

//Writes the user code formatted with "ExecBefore" and "ExecAfter" to a file
func writeCodeToFile(data *ProblemData, index int) {
	f, err := os.OpenFile(pyName, os.O_CREATE|os.O_WRONLY, 0755)
	defer f.Close()
	if err != nil {
		fmt.Println(err)
	}

	f.Write([]byte(data.ExecBefore[index]))
	f.Write([]byte(data.Code))
	f.Write([]byte(data.ExecAfter[index]))
}

//Run the process sending the correct input
func Run(ctx *context.Context, input string) (string, string, error) {
	cmd := exec.CommandContext(*ctx, "python",pyName)
	//	cmd := exec.CommandContext(*ctx, "python", fmt.Sprintf("./%v", pyName))
	var stdoutBuf, stderrBuf, inputBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	cmd.Stdin = &inputBuf
	inputBuf.WriteString(input)
	if err := cmd.Run(); err != nil {
		return stdoutBuf.String(), stderrBuf.String(), err
	}
	return stdoutBuf.String(), stderrBuf.String(), nil
}
