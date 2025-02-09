// Package mssql is the advisor for MSSQL database.
package mssql

import (
	"fmt"
	"regexp"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/tsql-parser"
	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/plugin/advisor"
	"github.com/bytebase/bytebase/backend/plugin/advisor/db"
	bbparser "github.com/bytebase/bytebase/backend/plugin/parser/sql"
)

var (
	_ advisor.Advisor = (*TableDropNamingConventionAdvisor)(nil)
)

func init() {
	advisor.Register(db.MSSQL, advisor.MSSQLTableDropNamingConvention, &TableDropNamingConventionAdvisor{})
}

// TableDropNamingConventionAdvisor is the advisor checking for table drop with naming convention..
type TableDropNamingConventionAdvisor struct {
}

// Check checks for table drop with naming convention..
func (*TableDropNamingConventionAdvisor) Check(ctx advisor.Context, _ string) ([]advisor.Advice, error) {
	tree, ok := ctx.AST.(antlr.Tree)
	if !ok {
		return nil, errors.Errorf("failed to convert to Tree")
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(ctx.Rule.Level)
	if err != nil {
		return nil, err
	}
	format, _, err := advisor.UnmarshalNamingRulePayloadAsRegexp(ctx.Rule.Payload)
	if err != nil {
		return nil, err
	}

	listener := &tableDropNamingConventionChecker{
		level:  level,
		title:  string(ctx.Rule.Type),
		format: format,
	}

	antlr.ParseTreeWalkerDefault.Walk(listener, tree)

	return listener.generateAdvice()
}

// tableDropNamingConventionChecker is the listener for table drop with naming convention.
type tableDropNamingConventionChecker struct {
	*parser.BaseTSqlParserListener

	level  advisor.Status
	title  string
	format *regexp.Regexp

	adviceList []advisor.Advice
}

// generateAdvice returns the advices generated by the listener, the advices must not be empty.
func (l *tableDropNamingConventionChecker) generateAdvice() ([]advisor.Advice, error) {
	if len(l.adviceList) == 0 {
		l.adviceList = append(l.adviceList, advisor.Advice{
			Status:  advisor.Success,
			Code:    advisor.Ok,
			Title:   "OK",
			Content: "",
		})
	}
	return l.adviceList, nil
}

// EnterDrop_table is called when production drop_table is entered.
func (l *tableDropNamingConventionChecker) EnterDrop_table(ctx *parser.Drop_tableContext) {
	allTableNames := ctx.AllTable_name()
	for _, tableName := range allTableNames {
		table := tableName.GetTable()
		if table == nil {
			continue
		}
		normalizedTableName := bbparser.NormalizeTSQLIdentifier(table)
		if !l.format.MatchString(normalizedTableName) {
			l.adviceList = append(l.adviceList, advisor.Advice{
				Status:  l.level,
				Code:    advisor.TableDropNamingConventionMismatch,
				Title:   l.title,
				Content: fmt.Sprintf("[%s] mismatches drop table naming convention, naming format should be %q", normalizedTableName, l.format),
				Line:    table.GetStart().GetLine(),
			})
		}
	}
}
