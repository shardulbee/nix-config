package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"filippo.io/age"
	"filippo.io/age/agessh"
	"golang.org/x/crypto/ssh"
)

var (
	secretsPath  string
	secretsID    string
	secretsFile  string
	secretsHosts string
	homeDir      string
)

func init() {
	secretsPath = os.Getenv("SECRETS_PATH")
	if secretsPath == "" {
		fmt.Fprintln(os.Stderr, "Error: SECRETS_PATH environment variable must be set")
		os.Exit(1)
	}

	var err error
	homeDir, err = os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	secretsID = filepath.Join(homeDir, ".ssh", "id_ed25519")
	secretsFile = filepath.Join(secretsPath, "secrets.age")
	secretsHosts = filepath.Join(secretsPath, "secrets.hosts")
}

func die(msg string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	os.Exit(1)
}

func checkDependencies() {
	// We still need sha256sum for hashing in edit command
	if _, err := exec.LookPath("sha256sum"); err != nil {
		die("sha256sum is not installed")
	}
}

func ensureSecretsID() {
	pubKeyPath := secretsID + ".pub"
	if _, err := os.Stat(pubKeyPath); os.IsNotExist(err) {
		fmt.Print("OK to generate a " + secretsID + " key? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		reply, _ := reader.ReadString('\n')
		reply = strings.TrimSpace(strings.ToLower(reply))

		if reply == "y" || reply == "yes" {
			fmt.Println("Generating secrets ID...")
			cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", secretsID, "-N", "")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				die("Failed to generate SSH key")
			}
			fmt.Println("Secrets ID generated")
			os.Exit(0)
		} else {
			die("Aborting")
		}
	}
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0600)
}

// Load SSH identity for age encryption/decryption
func loadSSHIdentity() (age.Identity, error) {
	privateKeyBytes, err := readFile(secretsID)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH key: %w", err)
	}

	identity, err := agessh.ParseIdentity(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH identity: %w", err)
	}

	return identity, nil
}

// Load SSH recipients from hosts file
func loadSSHRecipients() ([]age.Recipient, error) {
	hostsContent, err := readFile(secretsHosts)
	if err != nil {
		return nil, fmt.Errorf("failed to read hosts file: %w", err)
	}

	var recipients []age.Recipient
	lines := strings.Split(string(hostsContent), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse SSH public key
		pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
		if err != nil {
			continue // Skip invalid keys
		}

		var recipient age.Recipient
		recipient, err = agessh.NewRSARecipient(pubKey)
		if err != nil {
			// Try Ed25519
			recipient, err = agessh.NewEd25519Recipient(pubKey)
			if err != nil {
				continue // Skip unsupported key types
			}
		}
		recipients = append(recipients, recipient)
	}

	return recipients, nil
}

// Returns:
// 0 - Host can access secrets
// 1 - No secrets file exists yet
// 2 - Host key not in hosts file
// 3 - Host key cannot decrypt
func checkHostAccess() int {
	ensureSecretsID()

	// Check if neither secrets file nor hosts file exists
	_, secretsErr := os.Stat(secretsFile)
	_, hostsErr := os.Stat(secretsHosts)
	if os.IsNotExist(secretsErr) && os.IsNotExist(hostsErr) {
		fmt.Println("No secrets file exists yet. To get started:")
		fmt.Println("1. Run 'secrets add-this-host' on this machine to create your first key")
		fmt.Println("2. Run 'secrets edit' to create and encrypt your first secrets")
		return 1
	}

	// Read current host's public key
	pubKey, err := readFile(secretsID + ".pub")
	if err != nil {
		die("Failed to read public key")
	}

	// Check if this host's key is in the hosts file
	hostsContent, err := readFile(secretsHosts)
	if err != nil || !bytes.Contains(hostsContent, bytes.TrimSpace(pubKey)) {
		fmt.Println("This host is not authorized to access secrets.")
		fmt.Println()
		fmt.Println("To authorize this host:")
		fmt.Println("1. Run 'secrets add-this-host' to add this host's key")
		fmt.Println("2. Run 'secrets revalidate' on a machine that can already decrypt")
		return 2
	}

	// If secrets file exists, check if we can decrypt
	if _, err := os.Stat(secretsFile); err == nil {
		identity, err := loadSSHIdentity()
		if err != nil {
			die("Failed to load SSH identity")
		}

		// Try to decrypt
		encryptedFile, err := os.Open(secretsFile)
		if err != nil {
			die("Failed to open secrets file")
		}
		defer encryptedFile.Close()

		_, err = age.Decrypt(encryptedFile, identity)
		if err != nil {
			fmt.Println("This host's key is in the hosts file but cannot decrypt.")
			fmt.Println()
			fmt.Println("To fix this, either:")
			fmt.Println("1. Run 'secrets revalidate' on a machine that can decrypt to authorize this key")
			fmt.Println("2. Run 'secrets edit' on a machine that can decrypt, then try again")
			fmt.Println()
			fmt.Println("If you don't have access to a machine that can decrypt:")
			fmt.Println("Ask someone with access to run 'secrets revalidate' to authorize your key")
			return 3
		}
	}

	return 0
}

func decryptSecrets(outputFile string) error {
	identity, err := loadSSHIdentity()
	if err != nil {
		return fmt.Errorf("failed to load SSH identity: %w", err)
	}

	encryptedFile, err := os.Open(secretsFile)
	if err != nil {
		return fmt.Errorf("failed to open secrets file: %w", err)
	}
	defer encryptedFile.Close()

	decrypted, err := age.Decrypt(encryptedFile, identity)
	if err != nil {
		if checkHostAccess() != 0 {
			return fmt.Errorf("cannot decrypt secrets")
		}
		return err
	}

	// Read all decrypted content
	decryptedContent, err := io.ReadAll(decrypted)
	if err != nil {
		return fmt.Errorf("failed to read decrypted content: %w", err)
	}

	// Write to output file
	if err := writeFile(outputFile, decryptedContent); err != nil {
		return fmt.Errorf("failed to write decrypted content: %w", err)
	}

	return nil
}

func encryptSecrets(inputFile string) error {
	recipients, err := loadSSHRecipients()
	if err != nil {
		return fmt.Errorf("failed to load recipients: %w", err)
	}

	if len(recipients) == 0 {
		return fmt.Errorf("no valid recipients found in hosts file")
	}

	// Also add current identity as recipient
	_, err = loadSSHIdentity()
	if err != nil {
		return fmt.Errorf("failed to load SSH identity: %w", err)
	}

	// Read current public key to add as recipient
	pubKeyBytes, err := readFile(secretsID + ".pub")
	if err != nil {
		return fmt.Errorf("failed to read public key: %w", err)
	}

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(pubKeyBytes)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	var selfRecipient age.Recipient
	selfRecipient, err = agessh.NewRSARecipient(pubKey)
	if err != nil {
		// Try Ed25519
		selfRecipient, err = agessh.NewEd25519Recipient(pubKey)
		if err != nil {
			return fmt.Errorf("failed to create recipient from own key: %w", err)
		}
	}

	// Check if self is already in recipients
	selfInRecipients := false
	for _, r := range recipients {
		if fmt.Sprintf("%v", r) == fmt.Sprintf("%v", selfRecipient) {
			selfInRecipients = true
			break
		}
	}
	if !selfInRecipients {
		recipients = append(recipients, selfRecipient)
	}

	// Read input file
	plaintext, err := readFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	// Create output file
	out, err := os.Create(secretsFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	// Encrypt
	w, err := age.Encrypt(out, recipients...)
	if err != nil {
		return fmt.Errorf("failed to create encrypted writer: %w", err)
	}

	if _, err := w.Write(plaintext); err != nil {
		return fmt.Errorf("failed to write encrypted data: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close encrypted writer: %w", err)
	}

	return nil
}

func cmdList() {
	if checkHostAccess() != 0 {
		os.Exit(1)
	}

	tmpFile, err := os.CreateTemp("", "secrets")
	if err != nil {
		die("Failed to create temp file")
	}
	defer os.Remove(tmpFile.Name())

	if err := decryptSecrets(tmpFile.Name()); err != nil {
		die(fmt.Sprintf("Failed to decrypt: %v", err))
	}

	content, err := readFile(tmpFile.Name())
	if err != nil {
		die("Failed to read decrypted secrets")
	}
	fmt.Print(string(content))
}

func cmdActivate(shell string) {
	if checkHostAccess() != 0 {
		os.Exit(1)
	}

	tmpFile, err := os.CreateTemp("", "secrets")
	if err != nil {
		die("Failed to create temp file")
	}
	defer os.Remove(tmpFile.Name())

	if err := decryptSecrets(tmpFile.Name()); err != nil {
		die(fmt.Sprintf("Failed to decrypt: %v", err))
	}

	content, err := readFile(tmpFile.Name())
	if err != nil {
		die("Failed to read decrypted secrets")
	}

	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Process any KEY=value line
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				switch shell {
				case "fish":
					// Fish format - set -gx
					fmt.Printf("set -gx %s %s\n", key, value)
				case "bash", "zsh", "sh":
					// Bash/Zsh/sh format - export
					fmt.Printf("export %s=%s\n", key, value)
				default:
					die(fmt.Sprintf("Unsupported shell: %s. Supported shells: fish, bash, zsh, sh", shell))
				}
			}
		}
	}
}

func getFileHash(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(content)
	return fmt.Sprintf("%x", hash), nil
}

func cmdEdit() {
	tmpFile, err := os.CreateTemp("", "secrets")
	if err != nil {
		die("Failed to create temp file")
	}
	defer os.Remove(tmpFile.Name())

	// Special case for first-time setup
	if _, err := os.Stat(secretsFile); os.IsNotExist(err) {
		if checkHostAccess() > 1 {
			os.Exit(1)
		}
		fmt.Println("Creating new secrets file...")
		if err := writeFile(tmpFile.Name(), []byte("EXAMPLE_API_KEY=change_me\n")); err != nil {
			die("Failed to write initial content")
		}
	} else {
		if checkHostAccess() != 0 {
			os.Exit(1)
		}
		if err := decryptSecrets(tmpFile.Name()); err != nil {
			die(fmt.Sprintf("Failed to decrypt: %v", err))
		}
	}

	originalHash, err := getFileHash(tmpFile.Name())
	if err != nil {
		die("Failed to get file hash")
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nano"
	}

	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		die("Editor exited with error")
	}

	newHash, err := getFileHash(tmpFile.Name())
	if err != nil {
		die("Failed to get file hash")
	}

	if originalHash == newHash {
		fmt.Println("No changes made")
		os.Exit(0)
	}

	// Validate file format
	content, err := readFile(tmpFile.Name())
	if err != nil {
		die("Failed to read edited file")
	}
	// Validate file format - ensure all non-empty, non-comment lines are KEY=value format
	lines := strings.Split(string(content), "\n")
	hasValidLine := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.Contains(line, "=") {
			die(fmt.Sprintf("Invalid file format. All lines must be KEY=value format. Invalid line: %s", line))
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			die(fmt.Sprintf("Invalid file format. All lines must be KEY=value format. Invalid line: %s", line))
		}
		hasValidLine = true
	}
	if !hasValidLine {
		die("File must contain at least one KEY=value line")
	}

	// Encrypt the file
	if err := encryptSecrets(tmpFile.Name()); err != nil {
		die(fmt.Sprintf("Failed to encrypt: %v", err))
	}

	fmt.Println("Secrets updated successfully. Run the following to add to your shell:")
	fmt.Println("  secrets activate fish | source  # for fish shell")
	fmt.Println("  eval $(secrets activate bash)    # for bash shell")
	fmt.Println("  eval $(secrets activate zsh)     # for zsh shell")
}

func cmdRevalidate() {
	if checkHostAccess() != 0 {
		os.Exit(1)
	}

	tmpFile, err := os.CreateTemp("", "secrets")
	if err != nil {
		die("Failed to create temp file")
	}
	defer os.Remove(tmpFile.Name())

	if err := decryptSecrets(tmpFile.Name()); err != nil {
		die(fmt.Sprintf("Failed to decrypt: %v", err))
	}

	// Reencrypt with all hosts
	if err := encryptSecrets(tmpFile.Name()); err != nil {
		die(fmt.Sprintf("Failed to reencrypt: %v", err))
	}

	fmt.Println("Revalidation successful!")
	fmt.Println("File has been re-encrypted with all current host keys")
}

func cmdAddHost() {
	ensureSecretsID()

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(secretsHosts), 0755); err != nil {
		die("Failed to create secrets directory")
	}

	// Touch the hosts file if it doesn't exist
	if _, err := os.Stat(secretsHosts); os.IsNotExist(err) {
		if err := writeFile(secretsHosts, []byte{}); err != nil {
			die("Failed to create hosts file")
		}
	}

	// Read current public key
	currentKey, err := readFile(secretsID + ".pub")
	if err != nil {
		die("Failed to read public key")
	}
	currentKey = bytes.TrimSpace(currentKey)

	// Read existing hosts
	hostsContent, err := readFile(secretsHosts)
	if err != nil {
		die("Failed to read hosts file")
	}

	// Check if exact key already exists
	if bytes.Contains(hostsContent, currentKey) {
		fmt.Println("This exact key is already authorized")
		fmt.Println("Note: The key still needs to be validated by running 'secrets revalidate' on a machine that can decrypt")
		os.Exit(0)
	}

	// Extract hostname from key
	keyParts := strings.Fields(string(currentKey))
	if len(keyParts) < 3 {
		die("Invalid public key format")
	}
	currentHostname := keyParts[2]

	// Check for old keys from same host
	lines := strings.Split(string(hostsContent), "\n")
	var oldKeys []string
	for _, line := range lines {
		if strings.HasSuffix(line, " "+currentHostname) {
			oldKeys = append(oldKeys, line)
		}
	}

	if len(oldKeys) > 0 {
		fmt.Printf("Found existing key(s) for host '%s':\n", currentHostname)
		for _, key := range oldKeys {
			fmt.Println(key)
		}
		fmt.Println()
		fmt.Print("Remove old key(s) and add new one? [y/N] ")

		reader := bufio.NewReader(os.Stdin)
		reply, _ := reader.ReadString('\n')
		reply = strings.TrimSpace(strings.ToLower(reply))

		if reply == "y" || reply == "yes" {
			// Remove old keys
			var newLines []string
			for _, line := range lines {
				if !strings.HasSuffix(line, " "+currentHostname) && line != "" {
					newLines = append(newLines, line)
				}
			}
			// Add new key
			newLines = append(newLines, string(currentKey))

			// Write back
			newContent := strings.Join(newLines, "\n")
			if !strings.HasSuffix(newContent, "\n") {
				newContent += "\n"
			}
			if err := writeFile(secretsHosts, []byte(newContent)); err != nil {
				die("Failed to update hosts file")
			}
			fmt.Println("Old key(s) removed and new key added successfully")
		} else {
			fmt.Println("Operation cancelled")
			os.Exit(1)
		}
	} else {
		// Just append the new key
		if len(hostsContent) > 0 && !bytes.HasSuffix(hostsContent, []byte("\n")) {
			hostsContent = append(hostsContent, '\n')
		}
		hostsContent = append(hostsContent, currentKey...)
		hostsContent = append(hostsContent, '\n')

		if err := writeFile(secretsHosts, hostsContent); err != nil {
			die("Failed to update hosts file")
		}
		fmt.Println("Host key added successfully")
	}

	fmt.Println("Note: The key needs to be validated by running 'secrets revalidate' on a machine that can decrypt")
}

func cmdCheckHostAccess() {
	os.Exit(checkHostAccess())
}

func main() {
	checkDependencies()

	if len(os.Args) < 2 {
		fmt.Println("Usage: secrets <command>")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  list                Show raw decrypted secrets")
		fmt.Println("  activate <shell>    Output secrets for shell evaluation")
		fmt.Println("                      Shells: fish, bash, zsh, sh")
		fmt.Println("                      Usage: secrets activate fish | source")
		fmt.Println("  edit                Edit secrets in $EDITOR")
		fmt.Println("  add-this-host     Add current host's key to authorized hosts")
		fmt.Println("  revalidate        Reencrypt secrets with all current host keys")
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "list":
		cmdList()
	case "activate":
		if len(os.Args) < 3 {
			die("Usage: secrets activate <shell>\nSupported shells: fish, bash, zsh, sh")
		}
		cmdActivate(os.Args[2])
	case "edit":
		cmdEdit()
	case "add-this-host":
		cmdAddHost()
	case "revalidate":
		cmdRevalidate()
	case "check-host-access":
		cmdCheckHostAccess()
	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown command '%s'\n", cmd)
		os.Exit(1)
	}
}
