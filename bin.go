package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
)

type cmderr struct {
	code   int
	reason string
}

func (e *cmderr) Error() string {
	return e.reason
}

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		fmt.Println("usage: bin <command> [<args>]")
		fmt.Println()
		fmt.Println("available commands:")
		fmt.Println("  package   generates a zip containing binaries and checksums")
		fmt.Println("  checksum  generates a SHA256 checksum for a binary")
		fmt.Println("  validate  checks if a binary has a valid SHA256 checksum")
		os.Exit(0)
	}

	var err *cmderr
	switch args[0] {
	case "checksum":
		err = checksumCommand(args[1:])
	case "validate":
		err = validateCommand(args[1:])
	case "package":
		err = packageCommand(args[1:])
	case "inspect":
		err = inspectCommand(args[1:])
	case "install":
		err = installCommand(args[1:])
	default:
		err = &cmderr{1, fmt.Sprintf("%s is an unkown command.\n", args[0])}
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err.reason)
		os.Exit(err.code)
	}
}

func checksum(b []byte) string {
	hasher := sha256.New()
	hasher.Write(b)
	return fmt.Sprintf("sha256:%x", hasher.Sum(nil))
}

func installCommand(args []string) *cmderr {
	if len(args) < 1 {
		return &cmderr{1, "missing path to package as first argument"}
	}

	var pkg, sum string

	if !strings.Contains(args[0], ".package") {
		pkg = fmt.Sprintf("%s.package", args[0])
		sum = fmt.Sprintf("%s.checksum", args[0])
	} else {
		pkg = args[0]
		sum = fmt.Sprintf("%s.checksum", strings.Replace(args[0], ".package", "", 1))
	}

	//os, arch := runtime.GOOS, runtime.GOARCH

	if err := validateCommand([]string{pkg, sum}); err != nil {
		return err
	}

	file, err := os.Open(pkg)
	defer file.Close()

	if err != nil {
		return &cmderr{1, err.Error()}
	}

	gr, err := gzip.NewReader(file)
	defer gr.Close()

	if err != nil {
		return &cmderr{1, err.Error()}
	}

	tr := tar.NewReader(gr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return &cmderr{1, err.Error()}
		}

		content, err := io.ReadAll(tr)

		if err != nil {
			return &cmderr{1, err.Error()}
		}

		name := strings.Split(header.Name, ":")[2]

		if err := os.WriteFile(fmt.Sprintf(".bin/%s", name), content, fs.FileMode(header.Mode)); err != nil {
			return &cmderr{1, err.Error()}
		}
	}

	return nil
}

func inspectCommand(args []string) *cmderr {
	if len(args) < 1 {
		return &cmderr{1, "missing path to package as first argument"}
	}

	file, err := os.Open(args[0])
	defer file.Close()

	if err != nil {
		return &cmderr{1, err.Error()}
	}

	gr, err := gzip.NewReader(file)
	defer gr.Close()

	if err != nil {
		return &cmderr{1, err.Error()}
	}

	tr := tar.NewReader(gr)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return &cmderr{1, err.Error()}
		}
		fmt.Println(hdr.Name)
	}

	return nil
}

func packageCommand(args []string) *cmderr {
	if len(args) < 1 {
		return &cmderr{1, "missing path to folder or binary as first argument"}
	}

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	bin, err := os.Open(args[0])
	defer bin.Close()

	if err != nil {
		return &cmderr{1, err.Error()}
	}

	stat, err := os.Stat(args[0])

	if err != nil {
		return &cmderr{1, err.Error()}
	}

	b, err := io.ReadAll(bin)

	if err != nil {
		return &cmderr{1, err.Error()}
	}

	header := &tar.Header{
		Name: fmt.Sprintf("%s:%s", checksum(b), args[0]),
		Mode: int64(stat.Mode()),
		Size: int64(len(b)),
	}

	if err := tw.WriteHeader(header); err != nil {
		return &cmderr{1, err.Error()}
	}

	if _, err := tw.Write(b); err != nil {
		return &cmderr{1, err.Error()}
	}

	if err := tw.Close(); err != nil {
		return &cmderr{1, err.Error()}
	}

	if err := gw.Close(); err != nil {
		return &cmderr{1, err.Error()}
	}

	file, err := os.Create(fmt.Sprintf("%s.package", args[0]))
	defer file.Close()

	if err != nil {
		return &cmderr{1, err.Error()}
	}

	b = buf.Bytes()
	if _, err := file.Write(b); err != nil {
		return &cmderr{1, err.Error()}
	}

	file, err = os.Create(fmt.Sprintf("%s.checksum", args[0]))
	defer file.Close()

	if err != nil {
		return &cmderr{1, err.Error()}
	}

	if _, err := file.WriteString(fmt.Sprintf("%s\n", checksum(b))); err != nil {
		return &cmderr{1, err.Error()}
	}

	return nil
}

func checksumCommand(args []string) *cmderr {
	if len(args) < 1 {
		return &cmderr{1, "missing path to binary as first argument"}
	}

	bin, err := os.Open(args[0])

	if err != nil {
		return &cmderr{1, err.Error()}
	}

	b, err := io.ReadAll(bin)

	if err != nil {
		return &cmderr{1, err.Error()}
	}

	fmt.Println(checksum(b))
	return nil
}

func validateCommand(args []string) *cmderr {
	if len(args) < 1 {
		return &cmderr{1, "missing path to binary as first argument"}
	}

	if len(args) < 2 {
		return &cmderr{1, "missing path to checksum as second argument"}
	}

	bin, err := os.Open(args[0])

	if err != nil {
		return &cmderr{1, err.Error()}
	}

	ab, err := io.ReadAll(bin)

	if err != nil {
		return &cmderr{1, err.Error()}
	}

	checksumFile, err := os.Open(args[1])

	if err != nil {
		return &cmderr{1, err.Error()}
	}

	bb, err := io.ReadAll(checksumFile)

	if err != nil {
		return &cmderr{1, err.Error()}
	}

	if checksum(ab) != strings.Trim(string(bb), "\n") {
		return &cmderr{1, "invalid checksum for binary"}
	}

	return nil
}
