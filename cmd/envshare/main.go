// envshare is a friendly command line program for sharing env vars and
// secrets with a team, without ever putting readable secrets on a server
// or in a chat app.
//
// Every value is locked on YOUR own computer, to every current team
// member's public key, before it ever leaves your machine. The server
// only ever stores and forwards locked (encrypted) data.
//
// Everyday commands:
//
//	envshare keygen
//	envshare configure
//	envshare addmember
//	envshare removemember
//	envshare push .env staging
//	envshare push .env staging 30
//	envshare pull staging .env
//	envshare members
//	envshare environments
//	envshare history
//	envshare issues
//	envshare star
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"filippo.io/age"
)

// repoAddress is this project's home on GitHub. If this project is copied
// or forked under a different address, update this one line to match.
const repoAddress = "https://github.com/SohailKhan0525/envshare"

func openInBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func envshareDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".envshare")
}

func identityPath() string { return filepath.Join(envshareDir(), "identity.txt") }

type config struct {
	ServerURL string `json:"serverUrl"`
	Team      string `json:"team"`
	Token     string `json:"token"`
}

func configPath() string { return filepath.Join(envshareDir(), "config.json") }

func loadConfig() config {
	var c config
	data, err := os.ReadFile(configPath())
	if err == nil {
		_ = json.Unmarshal(data, &c)
	}
	return c
}

func saveConfig(c config) error {
	if err := os.MkdirAll(envshareDir(), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0o600)
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "envshare: "+format+"\n", args...)
	os.Exit(1)
}

var stdinReader = bufio.NewReader(os.Stdin)

func promptLine(label string) string {
	fmt.Print(label)
	line, _ := stdinReader.ReadString('\n')
	return strings.TrimSpace(line)
}

func loadIdentity() *age.X25519Identity {
	data, err := os.ReadFile(identityPath())
	if err != nil {
		fail("no personal key found yet at %s, please run the keygen command first", identityPath())
	}
	for _, l := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "AGE-SECRET-KEY-") {
			id, err := age.ParseX25519Identity(l)
			if err != nil {
				fail("could not read your personal key: %v", err)
			}
			return id
		}
	}
	fail("the file at %s did not contain a personal key", identityPath())
	return nil
}

func apiRequest(method, url, token string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(req)
}

func cmdKeygen() {
	path := identityPath()
	if _, err := os.Stat(path); err == nil {
		fail("you already have a personal key at %s, delete it first if you truly want a new one (you would lose access to anything already shared with you)", path)
	}
	id, err := age.GenerateX25519Identity()
	if err != nil {
		fail("could not create a key: %v", err)
	}
	if err := os.MkdirAll(envshareDir(), 0o700); err != nil {
		fail("could not create %s: %v", envshareDir(), err)
	}
	content := fmt.Sprintf("this file is your private key, keep it safe, never share it or upload it anywhere\n%s\n", id.String())
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		fail("could not save your key: %v", err)
	}
	fmt.Println("Your private key has been saved to:", path)
	fmt.Println()
	fmt.Println("Share this public key with your team admin so they can give you access:")
	fmt.Println()
	fmt.Println("  " + id.Recipient().String())
	fmt.Println()
	fmt.Println("Keep the private key file safe. Anyone who has it can read your team's secrets.")
}

func cmdConfigure() {
	cfg := loadConfig()
	fmt.Println("Let's set up envshare. Press enter to keep the value shown in brackets.")

	server := promptLine(fmt.Sprintf("Server address [%s]: ", cfg.ServerURL))
	if server != "" {
		cfg.ServerURL = server
	}
	team := promptLine(fmt.Sprintf("Team name [%s]: ", cfg.Team))
	if team != "" {
		cfg.Team = team
	}
	token := promptLine("Your personal access code (press enter to keep the saved one): ")
	if token != "" {
		cfg.Token = token
	}

	if err := saveConfig(cfg); err != nil {
		fail("could not save your settings: %v", err)
	}
	fmt.Println("Saved. You can now use push, pull, members, environments, and history without typing these again.")
}

func fetchMemberKeys(server, team, token string) ([]age.Recipient, error) {
	url := fmt.Sprintf("%s/api/teams/%s/members", strings.TrimRight(server, "/"), team)
	resp, err := apiRequest(http.MethodGet, url, token, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("the server said (%d): %s", resp.StatusCode, string(body))
	}
	var members []struct {
		Name      string `json:"name"`
		PublicKey string `json:"publicKey"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&members); err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, fmt.Errorf("this team has no members yet, add yourself first with the addmember command")
	}
	var recipients []age.Recipient
	for _, m := range members {
		r, err := age.ParseX25519Recipient(m.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("member %q has a public key that could not be read: %w", m.Name, err)
		}
		recipients = append(recipients, r)
	}
	return recipients, nil
}

func cmdAddMember() {
	cfg := loadConfig()
	if cfg.ServerURL == "" || cfg.Team == "" {
		fail("please run the configure command first, so envshare knows your server address and team name")
	}
	name := promptLine("New member's name (letters and numbers only): ")
	key := promptLine("New member's public key (starts with age1): ")
	adminToken := os.Getenv("EnvshareAdminToken")
	if adminToken == "" {
		adminToken = promptLine("Admin access code: ")
	}
	if name == "" || key == "" || adminToken == "" {
		fail("a name, a public key, and an admin access code are all required")
	}

	body, _ := json.Marshal(map[string]string{"name": name, "publicKey": key})
	url := fmt.Sprintf("%s/api/teams/%s/members", strings.TrimRight(cfg.ServerURL, "/"), cfg.Team)
	resp, err := apiRequest(http.MethodPost, url, adminToken, body)
	if err != nil {
		fail("the request failed: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		fail("the server said (%d): %s", resp.StatusCode, string(respBody))
	}
	var out struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(respBody, &out)
	fmt.Printf("Added %s. Send them this access code privately, for example in person or through a password manager, it will not be shown again:\n\n  %s\n", name, out.Token)
}

func cmdRemoveMember() {
	cfg := loadConfig()
	if cfg.ServerURL == "" || cfg.Team == "" {
		fail("please run the configure command first")
	}
	name := promptLine("Name of the member to remove: ")
	adminToken := os.Getenv("EnvshareAdminToken")
	if adminToken == "" {
		adminToken = promptLine("Admin access code: ")
	}
	if name == "" || adminToken == "" {
		fail("a name and an admin access code are both required")
	}
	url := fmt.Sprintf("%s/api/teams/%s/members/%s", strings.TrimRight(cfg.ServerURL, "/"), cfg.Team, name)
	resp, err := apiRequest(http.MethodDelete, url, adminToken, nil)
	if err != nil {
		fail("the request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fail("the server said (%d): %s", resp.StatusCode, string(body))
	}
	fmt.Printf("Removed %s.\n\nImportant: they can still read anything they already fetched. To fully lock them out of a secret, push a fresh copy of each environment now that they are off the list.\n", name)
}

func cmdPush(args []string) {
	if len(args) != 2 && len(args) != 3 {
		fail("please type: envshare push, then the file name, then the environment name, and optionally how many days until it expires, for example: envshare push .env staging or envshare push .env staging 30")
	}
	file, env := args[0], args[1]
	expireDays := 0
	if len(args) == 3 {
		n, err := strconv.Atoi(args[2])
		if err != nil || n <= 0 {
			fail("the expire days value must be a positive whole number, for example 30")
		}
		expireDays = n
	}
	cfg := loadConfig()
	if cfg.ServerURL == "" || cfg.Team == "" || cfg.Token == "" {
		fail("please run the configure command first")
	}

	plaintext, err := os.ReadFile(file)
	if err != nil {
		fail("could not read %s: %v", file, err)
	}
	recipients, err := fetchMemberKeys(cfg.ServerURL, cfg.Team, cfg.Token)
	if err != nil {
		fail("could not fetch the team's members: %v", err)
	}

	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recipients...)
	if err != nil {
		fail("could not start locking the file: %v", err)
	}
	if _, err := w.Write(plaintext); err != nil {
		fail("could not lock the file: %v", err)
	}
	if err := w.Close(); err != nil {
		fail("could not finish locking the file: %v", err)
	}

	url := fmt.Sprintf("%s/api/teams/%s/secrets/%s", strings.TrimRight(cfg.ServerURL, "/"), cfg.Team, env)
	if expireDays > 0 {
		url += fmt.Sprintf("?expireDays=%d", expireDays)
	}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(buf.Bytes()))
	if err != nil {
		fail("the request failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fail("the request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fail("the server said (%d): %s", resp.StatusCode, string(body))
	}
	if expireDays > 0 {
		fmt.Printf("Sent %s to team %s as environment %s, locked for %d member(s), expiring in %d day(s)\n", file, cfg.Team, env, len(recipients), expireDays)
	} else {
		fmt.Printf("Sent %s to team %s as environment %s, locked for %d member(s)\n", file, cfg.Team, env, len(recipients))
	}
}

func cmdPull(args []string) {
	if len(args) < 1 {
		fail("please type: envshare pull, then the environment name, for example: envshare pull staging")
	}
	env := args[0]
	out := ".env"
	if len(args) >= 2 {
		out = args[1]
	}
	cfg := loadConfig()
	if cfg.ServerURL == "" || cfg.Team == "" || cfg.Token == "" {
		fail("please run the configure command first")
	}

	url := fmt.Sprintf("%s/api/teams/%s/secrets/%s", strings.TrimRight(cfg.ServerURL, "/"), cfg.Team, env)
	resp, err := apiRequest(http.MethodGet, url, cfg.Token, nil)
	if err != nil {
		fail("the request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusGone {
		fail("this secret has expired, ask an admin to push a fresh copy")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fail("the server said (%d): %s", resp.StatusCode, string(body))
	}
	locked, err := io.ReadAll(resp.Body)
	if err != nil {
		fail("could not read the response: %v", err)
	}

	identity := loadIdentity()
	r, err := age.Decrypt(bytes.NewReader(locked), identity)
	if err != nil {
		fail("could not unlock this secret, you may not have access to it yet: %v", err)
	}
	plaintext, err := io.ReadAll(r)
	if err != nil {
		fail("could not read the unlocked data: %v", err)
	}
	if err := os.WriteFile(out, plaintext, 0o600); err != nil {
		fail("could not save %s: %v", out, err)
	}
	fmt.Printf("Saved team %s, environment %s, into %s\n", cfg.Team, env, out)
}

func cmdMembers() {
	cfg := loadConfig()
	if cfg.ServerURL == "" || cfg.Team == "" || cfg.Token == "" {
		fail("please run the configure command first")
	}
	url := fmt.Sprintf("%s/api/teams/%s/members", strings.TrimRight(cfg.ServerURL, "/"), cfg.Team)
	resp, err := apiRequest(http.MethodGet, url, cfg.Token, nil)
	if err != nil {
		fail("the request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fail("the server said (%d): %s", resp.StatusCode, string(body))
	}
	var members []struct {
		Name      string `json:"name"`
		PublicKey string `json:"publicKey"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&members)
	for _, m := range members {
		fmt.Printf("%-20s %s\n", m.Name, m.PublicKey)
	}
}

func cmdEnvironments() {
	cfg := loadConfig()
	if cfg.ServerURL == "" || cfg.Team == "" || cfg.Token == "" {
		fail("please run the configure command first")
	}
	url := fmt.Sprintf("%s/api/teams/%s/secrets", strings.TrimRight(cfg.ServerURL, "/"), cfg.Team)
	resp, err := apiRequest(http.MethodGet, url, cfg.Token, nil)
	if err != nil {
		fail("the request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fail("the server said (%d): %s", resp.StatusCode, string(body))
	}
	var names []string
	_ = json.NewDecoder(resp.Body).Decode(&names)
	if len(names) == 0 {
		fmt.Println("No environments have been pushed yet.")
		return
	}
	for _, n := range names {
		fmt.Println(n)
	}
}

func cmdHistory() {
	cfg := loadConfig()
	if cfg.ServerURL == "" || cfg.Team == "" || cfg.Token == "" {
		fail("please run the configure command first")
	}
	url := fmt.Sprintf("%s/api/teams/%s/history", strings.TrimRight(cfg.ServerURL, "/"), cfg.Team)
	resp, err := apiRequest(http.MethodGet, url, cfg.Token, nil)
	if err != nil {
		fail("the request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fail("the server said (%d): %s", resp.StatusCode, string(body))
	}
	var entries []struct {
		Time   string `json:"time"`
		Action string `json:"action"`
		Who    string `json:"who"`
		Detail string `json:"detail"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&entries)
	if len(entries) == 0 {
		fmt.Println("No activity recorded yet.")
		return
	}
	for _, e := range entries {
		fmt.Printf("%-25s %-18s %-12s %s\n", e.Time, e.Action, e.Who, e.Detail)
	}
}

func cmdIssues() {
	url := repoAddress + "/issues"
	fmt.Println("Opening the issues page in your browser:")
	fmt.Println("  " + url)
	if err := openInBrowser(url); err != nil {
		fmt.Println("Could not open a browser automatically, please visit the address above yourself.")
	}
}

func cmdStar() {
	fmt.Println("Opening the project page in your browser:")
	fmt.Println("  " + repoAddress)
	fmt.Println("If you find this useful, tap the star button near the top of the page, it genuinely helps other teams find and trust the project.")
	if err := openInBrowser(repoAddress); err != nil {
		fmt.Println("Could not open a browser automatically, please visit the address above yourself.")
	}
}

func usage() {
	fmt.Println(`envshare, locked env vars and secrets, shared safely with your team

Everyday commands, typed in plain order, no symbols needed:

  envshare keygen
      make your own personal key, only needs to be done once

  envshare configure
      tell envshare your server address, team name, and access code

  envshare addmember
      admin only, gives someone new access to the team's secrets

  envshare removemember
      admin only, takes someone off the team's access list

  envshare push .env staging
      lock the file named dot env and send it as the staging environment

  envshare push .env staging 30
      the same, but the secret automatically expires in 30 days

  envshare pull staging .env
      fetch the staging environment and save it into a file named dot env

  envshare members
      list everyone who currently has access

  envshare environments
      list every environment that has been shared so far

  envshare history
      show a plain log of who pushed, pulled, or changed what, and when

  envshare issues
      open this project's issues page, for reporting a problem or an idea

  envshare star
      open this project's page, so you can show your support with a star

Type envshare followed by any of these words to get started.`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "keygen":
		cmdKeygen()
	case "configure":
		cmdConfigure()
	case "addmember":
		cmdAddMember()
	case "removemember":
		cmdRemoveMember()
	case "push":
		cmdPush(os.Args[2:])
	case "pull":
		cmdPull(os.Args[2:])
	case "members":
		cmdMembers()
	case "environments":
		cmdEnvironments()
	case "history":
		cmdHistory()
	case "issues":
		cmdIssues()
	case "star":
		cmdStar()
	case "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "envshare does not know the word %q\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

