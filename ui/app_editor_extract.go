package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"requestflow/model"
)

func (a *App) editorExtract(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "提取变量").FontSize(12).Bold()
		ui.Spacer(c)
		if ui.Button(c, "+ 提取").Clicked() {
			step.Extract = append(step.Extract, model.Extract{Source: "body", Kind: "jsonpath"})
		}
	})
	fieldHint(c, t, "来源选 body/header/cookie；jsonpath 填 data.token，regex 填正则；css 用选择器多元素提数组；cssRows 用「行选择器+行内子字段」一次提取对象数组（标题+链接配对零错位）。expectMin 可防反爬空页。")
	for i := range step.Extract {
		ex := &step.Extract[i]
		idx := i
		ui.Column(c.Key(fmt.Sprintf("ex-%d", idx))).Padding(8).Gap(6).Background(t.Surface).Radius(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
				ui.TextInput(c, &ex.Name).Placeholder("变量名，如 token").Grow(1).Label("变量名")
				if ui.Button(c.Key("del"), "移除").Clicked() {
					step.Extract = append(step.Extract[:idx], step.Extract[idx+1:]...)
				}
			})
			// 来源 / 方式 / 默认值 都是短控件，同一行排列。
			ui.Row(c).Gap(8).Children(func() {
				sources := []string{"body", "header", "cookie", "status"}
				ui.Select(c, &ex.Source, sources).Label("来源").Grow(1)
				kinds := []string{"jsonpath", "regex", "whole", "css", "cssRows"}
				ui.Select(c, &ex.Kind, kinds).Label("方式").Grow(1)
				ui.TextInput(c, &ex.Default).Placeholder("可选").Label("默认值").Grow(1)
			})
			kind := strings.ToLower(strings.TrimSpace(ex.Kind))
			placeholder := "data.token 或正则"
			if kind == "css" {
				placeholder = "CSS 选择器，如 li.item a.title"
			} else if kind == "cssrows" {
				placeholder = "行选择器，如 div.item（每行提取多个字段）"
			}
			ui.TextInput(c, &ex.Path).Placeholder(placeholder).Label("路径 / 表达式")
			if kind == "css" {
				ui.Row(c).Gap(8).Children(func() {
					fields := []string{"text", "html", "attr"}
					if strings.TrimSpace(ex.CssField) == "" {
						ex.CssField = "text"
					}
					ui.Select(c, &ex.CssField, fields).Label("取值").Grow(1)
					if strings.EqualFold(strings.TrimSpace(ex.CssField), "attr") {
						ui.TextInput(c, &ex.CssAttr).Placeholder("href / content / ...").Label("属性名").Grow(2)
					}
				})
			}
			// cssRows：行内子字段编辑器（每个字段独立选择器与取值方式）。
			if kind == "cssrows" {
				a.editorCSSFields(c, t, ex)
			}
			// CSS / cssRows 通用：最少条数校验 + URL 绝对化/跳转解码。
			if kind == "css" || kind == "cssrows" {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					minStr := fmt.Sprint(ex.ExpectMin)
					if ui.TextInput(c, &minStr).Label("最少条数(0=不校验)").Width(160).Changed() {
						ex.ExpectMin, _ = strconv.Atoi(strings.TrimSpace(minStr))
					}
					ui.Checkbox(c, &ex.ResolveURL, "URL 绝对化/跳转解码")
				})
			}
		})
	}
}

// editorCSSFields 配置 cssRows 的行内子字段：每行一个字段名 + 行内选择器 + 取值方式(+属性名/+resolve)。

// editorCSSFields 配置 cssRows 的行内子字段：每行一个字段名 + 行内选择器 + 取值方式(+属性名/+resolve)。
func (a *App) editorCSSFields(c *ui.Context, t *ui.Theme, ex *model.Extract) {
	ui.Column(c).Padding(0, 8, 0, 8).Gap(4).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "行内子字段").FontSize(11).Bold().TextColor(t.TextMuted)
			ui.Spacer(c)
			if ui.Button(c, "+ 字段").Clicked() {
				ex.CssFields = append(ex.CssFields, model.CSSField{CssField: "text"})
			}
		})
		fieldHint(c, t, "字段名=输出对象的 key；行内选择器留空表示取行元素自身；attr 需填属性名。")
		for j := range ex.CssFields {
			f := &ex.CssFields[j]
			fj := j
			ui.Row(c.Key(fmt.Sprintf("csf-%d", fj))).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.TextInput(c, &f.Name).Placeholder("字段名,如 title").Width(120)
				ui.TextInput(c, &f.Path).Placeholder("行内选择器,如 h3").Grow(1)
				fields := []string{"text", "html", "attr"}
				if strings.TrimSpace(f.CssField) == "" {
					f.CssField = "text"
				}
				ui.Select(c, &f.CssField, fields).Width(80)
				if strings.EqualFold(strings.TrimSpace(f.CssField), "attr") {
					ui.TextInput(c, &f.CssAttr).Placeholder("href/mu").Width(90)
				}
				ui.Checkbox(c, &f.Resolve, "URL")
				if ui.Button(c.Key("del"), "×").Width(26).Clicked() {
					ex.CssFields = append(ex.CssFields[:fj], ex.CssFields[fj+1:]...)
				}
			})
		}
	})
}

func (a *App) editorAssertList(c *ui.Context, t *ui.Theme, step *model.Step) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "断言").FontSize(12).Bold()
		ui.Spacer(c)
		if ui.Button(c, "+ 断言").Clicked() {
			step.Assert = append(step.Assert, model.Assert{Source: "status", Op: "eq"})
		}
	})
	fieldHint(c, t, "校验响应是否符合预期：如 status eq 200、jsonpath 变量 contains 文本；失败即中止（除非勾选失败后继续）。")
	for i := range step.Assert {
		as := &step.Assert[i]
		idx := i
		ui.Column(c.Key(fmt.Sprintf("as-%d", idx))).Padding(8).Gap(6).Background(t.Surface).Radius(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
				ui.TextInput(c, &as.Name).Placeholder("断言说明").Grow(1).Label("名称")
				if ui.Button(c.Key("del"), "移除").Clicked() {
					step.Assert = append(step.Assert[:idx], step.Assert[idx+1:]...)
				}
			})
			// 来源 / 操作 同一行。
			ui.Row(c).Gap(8).Children(func() {
				sources := []string{"status", "jsonpath", "body", "header", "variable"}
				ui.Select(c, &as.Source, sources).Label("来源").Grow(1)
				ops := []string{"eq", "ne", "contains", "exists", "gt", "gte", "lt", "lte"}
				ui.Select(c, &as.Op, ops).Label("操作").Grow(1)
			})
			// 路径 / 期望值 同一行。
			ui.Row(c).Gap(8).Children(func() {
				ui.TextInput(c, &as.Path).Placeholder("jsonpath / header 名 / 变量名").Label("路径").Grow(1)
				ui.TextInput(c, &as.Value).Placeholder("期望值").Label("期望值").Grow(1)
			})
		})
	}
}

// editorBranch 条件分支编辑器：可视化拼装条件 + then/else 子步骤。
