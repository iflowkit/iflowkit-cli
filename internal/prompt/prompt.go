package prompt

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type IO struct {
	in  *bufio.Reader
	out io.Writer
}

func NewIO(in io.Reader, out io.Writer) *IO {
	return &IO{in: bufio.NewReader(in), out: out}
}

func (p *IO) AskString(label string, current *string, validate func(string) error) (string, error) {
	for {
		if current != nil && *current != "" {
			fmt.Fprintf(p.out, "%s [%s]: ", label, *current)
		} else {
			fmt.Fprintf(p.out, "%s: ", label)
		}
		line, err := p.readLine()
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(line) == "" && current != nil && *current != "" {
			line = *current
		}
		line = strings.TrimSpace(line)
		if validate != nil {
			if err := validate(line); err != nil {
				fmt.Fprintf(p.out, "  Error: %v\n", err)
				continue
			}
		}
		return line, nil
	}
}

func (p *IO) AskInt(label string, current *int, validate func(int) error) (int, error) {
	for {
		if current != nil {
			fmt.Fprintf(p.out, "%s [%d]: ", label, *current)
		} else {
			fmt.Fprintf(p.out, "%s: ", label)
		}
		line, err := p.readLine()
		if err != nil {
			return 0, err
		}
		line = strings.TrimSpace(line)
		if line == "" && current != nil {
			v := *current
			if validate != nil {
				if err := validate(v); err != nil {
					fmt.Fprintf(p.out, "  Error: %v\n", err)
					continue
				}
			}
			return v, nil
		}
		v, err := strconv.Atoi(line)
		if err != nil {
			fmt.Fprintf(p.out, "  Error: please enter a number\n")
			continue
		}
		if validate != nil {
			if err := validate(v); err != nil {
				fmt.Fprintf(p.out, "  Error: %v\n", err)
				continue
			}
		}
		return v, nil
	}
}

func (p *IO) AskYesNo(label string, defaultYes bool) (bool, error) {
	def := "y/N"
	if defaultYes {
		def = "Y/n"
	}
	for {
		fmt.Fprintf(p.out, "%s (%s): ", label, def)
		line, err := p.readLine()
		if err != nil {
			return false, err
		}
		line = strings.TrimSpace(strings.ToLower(line))
		if line == "" {
			return defaultYes, nil
		}
		switch line {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Fprintf(p.out, "  Error: please answer yes or no\n")
		}
	}
}

func (p *IO) readLine() (string, error) {
	line, err := p.in.ReadString('\n')
	if err == io.EOF {
		return strings.TrimRight(line, "\r\n"), nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
