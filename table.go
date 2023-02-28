package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	newlineCharLength = 1
)

const (
	Addition       = '+'
	Subtraction    = '-'
	Multiplication = '*'
	Division       = '/'
)

var (
	formulaRegExCompiled    *regexp.Regexp
	columnNameRegExCompiled *regexp.Regexp
	rowNameRegExCompiled    *regexp.Regexp
)

func init() {
	formulaRegExCompiled, _ = regexp.Compile(`^=((?P<arg1ColName>[a-zA-Z]+)(?P<arg1RowName>\d+)|(?P<arg1Const>\d+))(?P<operator>[+*/-])((?P<arg2ColName>[a-zA-Z]+)(?P<arg2RowName>\d+)|(?P<arg2Const>\d+))$`)
	columnNameRegExCompiled, _ = regexp.Compile(`^[a-zA-Z]+$`)
	rowNameRegExCompiled, _ = regexp.Compile(`^\d+$`)
}

type Table struct {
	RowNamesMap map[string]int
	ColNamesMap map[string]int
	ColNames    []string
	RowNames    []string
	RowCount    int
	ColCount    int
	Comma       string
	cells       [][]string
	formulas    map[Address]*Formula
}

type Formula struct {
	Operand1Addr *Address
	Operand2Addr *Address
	Operator     rune
	Arg1Const    int
	Arg2Const    int
	isCalculated bool
	result       int
}

type Address struct {
	ColumnIndex int
	RowIndex    int
}

func (t *Table) ParseTable(filePath string) error {
	if !strings.HasSuffix(filePath, ".csv") {
		return fmt.Errorf("неправильный формат файла")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return err
	}

	t.Comma = string(reader.Comma)

	t.RowCount = len(records) - 1
	t.ColCount = len(records[0]) - 1
	t.cells = make([][]string, t.RowCount)
	for row := range t.cells {
		t.cells[row] = records[row+1][1:]
	}

	t.ColNamesMap = make(map[string]int)
	t.RowNamesMap = make(map[string]int)
	t.ColNames = make([]string, t.ColCount)
	t.RowNames = make([]string, t.RowCount)
	t.formulas = make(map[Address]*Formula)

	for i, colName := range records[0][1:] {
		t.ColNamesMap[colName] = i
		t.ColNames[i] = colName
	}
	for i := 0; i < len(t.cells); i++ {
		t.RowNamesMap[records[i+1][0]] = i
		t.RowNames[i] = records[i+1][0]
	}

	err = t.validateTableKeys(&t.ColNames, columnNameRegExCompiled)
	if err != nil {
		return err
	}
	err = t.validateTableKeys(&t.RowNames, rowNameRegExCompiled)
	if err != nil {
		return err
	}

	for rowIndex := 0; rowIndex < len(t.cells); rowIndex++ {
		for colIndex := 0; colIndex < len(t.cells[rowIndex]); colIndex++ {
			err = t.parseRecord(&t.cells[rowIndex][colIndex], &Address{ColumnIndex: colIndex, RowIndex: rowIndex})
			if err != nil {
				return fmt.Errorf("Col %d Row %d - %v\n", colIndex, rowIndex, err.Error())
			}
		}
	}

	var calculatedValue int
	for addr, formula := range t.formulas {
		calculatedValue, err = t.calculateFormula(formula)

		if err != nil {
			return err
		}

		t.cells[addr.RowIndex][addr.ColumnIndex] = strconv.Itoa(calculatedValue)
	}

	return file.Close()
}

func (t *Table) validateTableKeys(keys *[]string, regexChecker *regexp.Regexp) error {
	uniqueKeys := make([]string, len(*keys))
	uniqueKeysLen := 0

	for _, key := range *keys {
		if !regexChecker.MatchString(key) {
			return fmt.Errorf("названия колонок должны состоять из латинских букв, а строк - из арабских чисел")
		}

		for i := 0; i < uniqueKeysLen; i++ {
			if key == uniqueKeys[i] {
				return fmt.Errorf("ключи должны состоять из уникальных значений")
			}
		}
		uniqueKeys[uniqueKeysLen] = key
		uniqueKeysLen++
	}
	return nil
}

func (t *Table) parseRecord(value *string, address *Address) error {
	if strings.HasPrefix(*value, "=") {
		matches := formulaRegExCompiled.FindStringSubmatch(*value)
		if matches != nil {
			var arg1ColName, arg1RowName, arg2ColName, arg2RowName string
			var operator rune
			var arg1Const, arg2Const int

			for i, name := range formulaRegExCompiled.SubexpNames() {
				if i != 0 && name != "" {
					switch name {
					case "arg1ColName":
						arg1ColName = matches[i]
					case "arg1RowName":
						arg1RowName = matches[i]
					case "arg2ColName":
						arg2ColName = matches[i]
					case "arg2RowName":
						arg2RowName = matches[i]
					case "arg1Const":
						arg1Const, _ = strconv.Atoi(matches[i])
					case "arg2Const":
						arg2Const, _ = strconv.Atoi(matches[i])
					case "operator":
						operator = rune(matches[i][0])
					}
				}
			}

			var address1, address2 *Address
			if arg1ColName != "" && arg1RowName != "" {
				err := t.validateCellAddress(&arg1ColName, &arg1RowName)
				if err != nil {
					return err
				}

				address1 = &Address{ColumnIndex: t.ColNamesMap[arg1ColName], RowIndex: t.RowNamesMap[arg1RowName]}
			}
			if arg2ColName != "" && arg2RowName != "" {
				err := t.validateCellAddress(&arg2ColName, &arg2RowName)
				if err != nil {
					return err
				}

				address2 = &Address{ColumnIndex: t.ColNamesMap[arg2ColName], RowIndex: t.RowNamesMap[arg2RowName]}
			}

			t.formulas[*address] = &Formula{
				Operand1Addr: address1,
				Operand2Addr: address2,
				Operator:     operator,
				Arg1Const:    arg1Const,
				Arg2Const:    arg2Const,
			}

		} else {
			return fmt.Errorf("некорректная формула")
		}

	} else {
		_, err := strconv.Atoi(*value)
		if err != nil {
			err = fmt.Errorf("ошибка во время парсинга целочисленных значений")
		}
		return err
	}

	return nil
}

func (t *Table) validateCellAddress(columnName, rowName *string) error {
	for existColName := range t.ColNamesMap {
		if *columnName == existColName {
			for existRowName := range t.RowNamesMap {
				if *rowName == existRowName {
					return nil
				}
			}
		}
	}
	return fmt.Errorf("некорректный адрес в формуле")
}

func (t *Table) calculateFormula(formula *Formula) (int, error) {
	if formula.isCalculated {
		return formula.result, nil
	}

	var operand1, operand2 int

	if formula.Operand1Addr == nil {
		operand1 = formula.Arg1Const
	} else {
		operand1, _ = t.GetCellNumericValueByAddress(formula.Operand1Addr)
	}

	if formula.Operand2Addr == nil {
		operand2 = formula.Arg2Const
	} else {
		operand2, _ = t.GetCellNumericValueByAddress(formula.Operand2Addr)
	}

	result := 0
	switch formula.Operator {
	case Addition:
		result = operand1 + operand2
	case Subtraction:
		result = operand1 - operand2
	case Multiplication:
		result = operand1 * operand2
	case Division:
		if operand2 == 0 {
			return 0, fmt.Errorf("деление на ноль")
		} else {
			result = operand1 / operand2
		}
	default:
		return 0, fmt.Errorf("недопустимый оператор")
	}

	return result, nil
}

func (t *Table) GetCellNumericValueByAddress(address *Address) (int, error) {
	rawValue, err := t.GetCellValueByAddress(address)
	if err != nil {
		return 0, err
	}

	result := 0

	if strings.HasPrefix(rawValue, "=") {
		result, err = t.calculateFormula(t.formulas[*address])
		if err != nil {
			return 0, err
		}
	} else {
		result, _ = strconv.Atoi(rawValue)
	}

	return result, nil
}

func (t *Table) GetCellValueByAddress(address *Address) (string, error) {
	if address.ColumnIndex < 0 || address.RowIndex < 0 || address.ColumnIndex >= len(t.cells[0]) || address.RowIndex >= len(t.cells) {
		return "", fmt.Errorf("некорректный адрес")
	}
	return t.cells[address.RowIndex][address.ColumnIndex], nil
}

func (t *Table) String() string {
	var builder strings.Builder
	builder.Grow(t.countTableStringLength())

	for _, colName := range t.ColNames {
		builder.WriteString(t.Comma)
		builder.WriteString(colName)
	}

	for rowIndex, rowName := range t.RowNames {
		builder.WriteString("\n")
		builder.WriteString(rowName)
		for colIndex := 0; colIndex < len(t.cells[rowIndex]); colIndex++ {
			builder.WriteString(t.Comma)
			builder.WriteString(t.cells[rowIndex][colIndex])
		}
	}

	return builder.String()
}

func (t *Table) countTableStringLength() int {
	var tableLength int
	separatorLength := len(t.Comma)

	for _, colName := range t.ColNames {
		tableLength += len(colName) + separatorLength
	}
	tableLength += newlineCharLength

	for _, rowName := range t.RowNames {
		tableLength += len(rowName)
	}

	for rowIndex := 0; rowIndex < len(t.cells); rowIndex++ {
		for colIndex := 0; colIndex < len(t.cells[rowIndex]); colIndex++ {
			tableLength += len(t.cells[rowIndex][colIndex]) + separatorLength
		}
		tableLength += newlineCharLength
	}

	return tableLength
}
