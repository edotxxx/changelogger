package changelogger

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"
)

type App struct {
	args   []string
	input  *bufio.Reader
	output io.Writer
	now    func() time.Time
	git    Git
}

func NewApp(args []string, input io.Reader, output io.Writer, now func() time.Time, runner Runner) App {
	return App{
		args:   args,
		input:  bufio.NewReader(input),
		output: output,
		now:    now,
		git:    Git{runner: runner},
	}
}

func (app App) Run() (runErr error) {
	restoreSourceBranch := true

	config, err := LoadConfig(".env")
	if err != nil {
		return err
	}

	if len(app.args) > 0 {
		config.RepositoryLink = app.args[0]
	}

	sourceBranch, err := app.askSourceBranch()
	if err != nil {
		return err
	}
	defer func() {
		if !restoreSourceBranch {
			return
		}

		if err := app.git.Checkout(sourceBranch); err != nil && runErr == nil {
			runErr = err
		}
	}()

	lastTag, err := app.git.LastTag()
	if err != nil {
		return err
	}

	version, err := ParseVersion(lastTag)
	if err != nil {
		return err
	}

	masterCommit, err := app.git.MasterCommit()
	if err != nil {
		return err
	}

	branchName, err := app.askBranchName(config.BranchPrefix)
	if err != nil {
		return err
	}

	if err := app.git.CreateBranch(branchName, sourceBranch); err != nil {
		return err
	}

	app.printColored(fmt.Sprintf("Текущая версия приложения: %s\n", version.String()), yellow)
	app.print("Какую версию нужно поднять?\n 1 - major (*.0.0)\n 2 - minor (0.*.0)\n 3 - fix   (0.0.*)  - ")

	level, err := app.readLine()
	if err != nil {
		return err
	}

	newVersion, err := version.Next(level)
	if err != nil {
		return err
	}

	app.printColored(fmt.Sprintf("Следующая версия приложения: %s \n", newVersion.String()), green)

	commitLines, err := app.git.ChangeLines(version.String(), sourceBranch, masterCommit)
	if err != nil {
		return err
	}

	changelog := NewChangelog(config, app.now)
	answerBody := changelog.Body(commitLines)
	if answerBody == "" {
		return fmt.Errorf("отсутствуют коммиты с нужными тэгами")
	}

	app.printColored("Изменения которые попадут в CHANGELOG.md: \n"+answerBody+" \n", yellow)

	if err := app.askConfirmation("Все верно?"); err != nil {
		return err
	}
	restoreSourceBranch = false

	if err := changelog.Write(config.ChangelogPath, newVersion.String(), answerBody); err != nil {
		return err
	}
	app.printColored("Файл CHANGELOG.md успешно отредактирован:  \n", green)

	createCommit, err := app.askYesNo("Создать коммит?")
	if err != nil {
		return err
	}
	if !createCommit {
		app.printColored("Ветка оставлена для ручных правок. Коммит не создан.\n", yellow)
		return nil
	}

	if err := app.git.Commit(config.ChangelogPath); err != nil {
		return err
	}
	app.printColored("Коммит успешно создан!  \n", green)

	pushBranch, err := app.askYesNo("Пушить ветку " + branchName + "?")
	if err != nil {
		return err
	}
	if !pushBranch {
		app.printColored("Ветка оставлена для ручных правок. Push не выполнен.\n", yellow)
		return nil
	}

	if err := app.git.Push(branchName); err != nil {
		return err
	}

	return app.git.DeleteBranch(branchName, sourceBranch)
}

func (app App) askSourceBranch() (string, error) {
	currentBranch, err := app.git.CurrentBranch()
	if err != nil {
		currentBranch = ""
	}

	releaseBranches, err := app.git.ReleaseBranches(10)
	if err != nil {
		return "", err
	}

	app.print("Из какой ветки собрать changelog?\n\n")

	options := map[string]string{"1": "develop"}
	optionNumber := 1
	app.print("1 - develop [по умолчанию]\n")

	for _, branch := range releaseBranches {
		if branch == "develop" {
			continue
		}

		optionNumber++
		key := fmt.Sprint(optionNumber)
		options[key] = branch
		app.print(fmt.Sprintf("%s - %s\n", key, branch))
	}

	if isAssignToChangelogBranch(currentBranch) {
		app.printColored(fmt.Sprintf("Текущая ветка %s служебная, она не будет предложена как источник changelog.\n", currentBranch), yellow)
	} else if currentBranch != "develop" && !branchInOptions(currentBranch, options) {
		optionNumber++
		key := fmt.Sprint(optionNumber)
		options[key] = currentBranch
		app.print(fmt.Sprintf("%s - текущая ветка: %s\n", key, currentBranch))
	}

	app.print("0 - ввести вручную\n\n")
	app.print("Выбор [1]: ")

	input, err := app.readLine()
	if err != nil {
		return "", err
	}
	if input == "" {
		return "develop", nil
	}
	if input == "0" {
		return app.askManualSourceBranch()
	}

	sourceBranch, ok := options[input]
	if !ok {
		return "", fmt.Errorf("неверный выбор ветки-источника")
	}

	return app.prepareSourceBranch(sourceBranch)
}

func (app App) askManualSourceBranch() (string, error) {
	app.print("Введите ветку: ")

	branch, err := app.readLine()
	if err != nil {
		return "", err
	}

	if branch == "" {
		return "", fmt.Errorf("ветка-источник не указана")
	}
	if isAssignToChangelogBranch(branch) {
		return "", fmt.Errorf("ветка %s служебная и не может быть источником changelog", branch)
	}

	return app.prepareSourceBranch(branch)
}

func (app App) prepareSourceBranch(branch string) (string, error) {
	if !strings.HasPrefix(branch, "origin/") {
		return branch, nil
	}

	return app.git.CheckoutRemoteBranch(branch)
}

func (app App) askBranchName(prefix string) (string, error) {
	app.printColored(`Введите номер заявки и задачи (в формате Заявка-Задача, например "IU888000-W0999000"): `, yellow)

	input, err := app.readLine()
	if err != nil {
		return "", err
	}

	if !validBranchTask(input) {
		return "", fmt.Errorf(`неверный формат. Ожидался формат Заявка-Задача, например "IU888000-W0999000"`)
	}

	if prefix != "" {
		prefix += "-"
	}

	return "feature/" + prefix + input + "-assign-to-changelog", nil
}

func (app App) askConfirmation(question string) error {
	app.print(question + " (y/n): ")

	answer, err := app.readLine()
	if err != nil {
		return err
	}

	switch strings.ToLower(answer) {
	case "y", "yes":
		return nil
	default:
		return fmt.Errorf("выполнение команды отменено")
	}
}

func (app App) askYesNo(question string) (bool, error) {
	app.print(question + " (y/n): ")

	answer, err := app.readLine()
	if err != nil {
		return false, err
	}

	switch strings.ToLower(answer) {
	case "y", "yes":
		return true, nil
	case "n", "no", "н", "нет":
		return false, nil
	default:
		return false, fmt.Errorf("неверный ответ")
	}
}

func (app App) readLine() (string, error) {
	line, err := app.input.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}

	return strings.TrimSpace(line), nil
}

func (app App) print(message string) {
	fmt.Fprint(app.output, message)
}

func (app App) printColored(message string, color string) {
	fmt.Fprintf(app.output, "\033[01;%sm%s\033[0m", color, message)
}

func branchInOptions(branch string, options map[string]string) bool {
	for _, optionBranch := range options {
		if optionBranch == branch {
			return true
		}
	}

	return false
}

const (
	green  = "32"
	yellow = "33"
)
