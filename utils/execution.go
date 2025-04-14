package utils

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv" // Add this import
	"strings"
)

// GetSystemInfo returns information about the operating system
// This is the canonical implementation
func GetSystemInfo() string {
	return runtime.GOOS + " - " + runtime.GOARCH
}

// GetDirectoryStructure returns a string representation of the current directory structure
// with improved performance for large directories
// This is the canonical implementation
func GetDirectoryStructure(maxDepth int) string {
	if maxDepth <= 0 {
		maxDepth = 2
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "Error getting current directory"
	}

	var result strings.Builder
	fileCount := 0
	maxFiles := 100 // Limit to prevent excessive output

	err = filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors and continue
		}

		// Stop if we've reached the file limit
		if fileCount >= maxFiles {
			return filepath.SkipDir
		}

		// Calculate depth
		relPath, err := filepath.Rel(cwd, path)
		if err != nil {
			return nil
		}

		if relPath == "." {
			return nil
		}

		depth := len(strings.Split(relPath, string(os.PathSeparator)))
		if depth > maxDepth {
			if info.IsDir() {
				return filepath.SkipDir // Skip deeper directories
			}
			return nil
		}

		prefix := strings.Repeat("  ", depth-1)
		if info.IsDir() {
			result.WriteString(prefix + "📁 " + info.Name() + "\n")
		} else {
			result.WriteString(prefix + "📄 " + info.Name() + "\n")
		}

		fileCount++
		return nil
	})

	if err != nil {
		return "Error reading directory structure"
	}

	// Add a message if we hit the file limit
	if fileCount >= maxFiles {
		// Fix the integer to string conversion using strconv.Itoa
		result.WriteString("\n... (output limited to " + strconv.Itoa(maxFiles) + " entries)\n")
	}

	return result.String()
}

// // walkDir is a helper function to recursively walk directories
// func walkDir(path, indent string, depth, maxDepth int, result *strings.Builder) {
// 	if depth > maxDepth {
// 		return
// 	}

// 	info, err := os.Stat(path)
// 	if err != nil {
// 		return
// 	}

// 	// Add this file/directory to the result
// 	result.WriteString(fmt.Sprintf("%s%s\n", indent, filepath.Base(path)))

// 	// If it's a directory, recursively walk its contents
// 	if info.IsDir() {
// 		// Read directory contents
// 		entries, err := os.ReadDir(path)
// 		if err != nil {
// 			return
// 		}

// 		// Sort entries (directories first, then files)
// 		dirs := []os.DirEntry{}
// 		files := []os.DirEntry{}

// 		for _, entry := range entries {
// 			// Skip hidden files/directories
// 			if strings.HasPrefix(entry.Name(), ".") {
// 				continue
// 			}

// 			if entry.IsDir() {
// 				dirs = append(dirs, entry)
// 			} else {
// 				files = append(files, entry)
// 			}
// 		}

// 		// Process directories
// 		for i, entry := range dirs {
// 			childIndent := indent + "├── "
// 			if i == len(dirs)-1 && len(files) == 0 {
// 				childIndent = indent + "└── "
// 			}
// 			walkDir(filepath.Join(path, entry.Name()), childIndent, depth+1, maxDepth, result)
// 		}

// 		// Process files
// 		for i, entry := range files {
// 			childIndent := indent + "├── "
// 			if i == len(files)-1 {
// 				childIndent = indent + "└── "
// 			}
// 			result.WriteString(fmt.Sprintf("%s%s\n", childIndent, entry.Name()))
// 		}
// 	}
// }

// ExecuteCommand runs a shell command and returns the output
func ExecuteCommand(command string) (string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("bash", "-c", command)
	}

	output, err := cmd.CombinedOutput()
	return string(output), err
}
