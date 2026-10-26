package editor

import (
	"encoding/json"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/internal/i18n"
)

var editorMessages = map[string]string{
	"保存请求未通过来源校验。请从当前 Flare 页面重试；使用反向代理时，请保留原始 Host 请求头及外部端口。关闭登录不会关闭来源校验。": "editor.write_source_rejected",
	"保存响应格式异常（HTTP {0}），请检查服务端或反向代理配置。":                                     "editor.unexpected_save_response",

	"编辑器已升级，请先保留当前内容，再刷新页面后重新编辑。":     "editor_b52d5d7c6596",
	"无法读取导航数据，请检查配置文件及日志。":            "editor_ce04abd43409",
	"请从 Flare 编辑器保存":                  "editor_ba365b678f44",
	"提交数据格式无效":                        "editor_93372134fcdb",
	"服务器数据已变化，当前修改未保存。请导出待保存数据或重新载入。": "editor_fd5c5004f389",
	"分类 ID 缺失或重复":                     "editor_f5e9bc3c39c2",
	"分类名称不能为空、重复或使用保留名称":              "editor_3290540aca4e",
	"应用不能引用书签分类":                      "editor_032808d1533b",
	"书签引用的分类不存在，请选择有效位置":              "editor_a48ff4574c54",
	"保存失败且恢复未完成，请检查服务端恢复副本。":          "editor_620891bbdbc8",
	"保存失败，已恢复已替换文件；请检查服务端恢复副本。":       "editor_d7d9c89d8ff3",
	"保存失败，请检查服务端日志后重试。":               "editor_585ce794dd45",
	"搜索文字过长": "editor_e42b46ec7f55",
	"无效分页位置": "editor_b51f356a12ff",

	"应用区":     "editor_3b868385bcba",
	"默认分类 · ": "editor_b313e29eba9b",
	"未分类书签":   "editor_2cce844225aa",
	"分类缺失：":   "editor_9af89e41b374",
	"全部位置":    "editor_7990e5fdd73a",
	"请选择位置":   "editor_a94bda623fb6",
	"保存恢复未完成，请先导出草稿并检查服务端恢复副本，再重新载入。": "editor_04afe66560c4",
	"服务器数据已变化，请保留修改后重新载入。":            "editor_23bde0a1f2da",
	"有未保存的更改":  "editor_6265f9d3f3cd",
	"所有更改已保存":  "editor_b9a307eafc9f",
	" 条":       "editor_6fb2840d568a",
	"没有找到匹配条目": "editor_88c7bd86494a",
	"还没有条目，从新增或导入开始。": "editor_9c7c4ec49138",
	"清除筛选":      "editor_657d9cbf45ec",
	"新增条目":      "editor_22f794060f30",
	"编辑":        "editor_051836569928",
	"删除":        "editor_2f9daa828907",
	"编辑条目":      "editor_2d8c735968f1",
	"放弃本次表单修改？": "editor_595ed900db8d",
	"请填写名称":     "editor_fbc79033bfa2",
	"请填写地址":     "editor_c8b4f6518e55",
	"请选择有效位置":   "editor_f4135f051022",
	"条目已移至“":    "editor_98ef2673949d",
	"”，保存后生效。":  "editor_236c727cfff6",
	"条目已移除，保存后生效；可撤销本次删除。": "editor_11eda8ab76f4",
	"还没有书签分类。":             "editor_c2fb9ff00ed0",
	"分类名称 ":                "editor_20272271082e",
	"名称不能为空、重复或使用保留名称":     "editor_d9dba021d92f",
	" 条书签":          "editor_a40b297d60a7",
	"上移":            "editor_f853a70b1204",
	"下移":            "editor_e75e8b4e5c97",
	"删除空分类“":        "editor_015fb623dd22",
	"包含 ":           "editor_d027cac94e86",
	" 条书签，请选择迁入位置。": "editor_aac9173eeea7",
	"迁入位置":          "editor_f83424f3602b",
	"取消":            "editor_2cd0f3be8738",
	"迁移并删除":         "editor_e2a2b55c88fa",
	"尚未添加的新分类名称将被放弃，继续？":  "editor_21e1d8e8b020",
	"分类名称或 ID 无效":         "editor_748238f0dcfa",
	"正在保存…":               "editor_6bdb4435095e",
	"登录可能已过期，请在新窗口登录后重试。": "editor_f7d4949c5d13",
	"保存失败，请重试。":           "editor_3461df8deb4d",
	"保存响应不完整，请检查数据后重试。":   "editor_fc37f48d2e42",
	"保存结果待确认：请求中断或响应无法读取，服务端可能已经保存。请先导出草稿，再检查服务端数据；重新保存仍会校验版本。 当前编辑内容已保留。": "editor_eb7d1a11f22d",
	" 当前编辑内容已保留。":                 "editor_98545cfb8ef7",
	"放弃全部未保存的更改？":                 "editor_9db9aab68891",
	"已恢复加载时的数据；服务器已有更新，请重新载入。":    "editor_52eb1e3a9988",
	"重新载入会放弃当前修改，请先导出需要保留的内容。继续？": "editor_da8b178797b8",
	"不是有效的编辑器草稿":                  "editor_b6666ab006fb",
	"分类格式无效":                      "editor_cc8f8d464c91",
	"条目格式无效":                      "editor_ce0a14580385",
	"以草稿替换当前待保存内容？保存时仍会检查本页加载的服务器版本。": "editor_554e478307f0",
	"草稿已恢复到待保存内容，请检查后保存。":             "editor_fa58a7a16cd6",
	"无法恢复草稿：":              "editor_eb9830140ff4",
	"更改尚未保存，确定离开？":         "editor_ba7bc0dccdb9",
	"无法读取导航数据，请重新登录或检查配置。": "editor_32796e5df7d3",
	"加载失败":            "editor_d1d044826a45",
	"条目不存在":           "editor_17314a19ac96",
	"请填写名称或删除空行":      "editor_90cba0682c9e",
	"请填写地址或删除空行":      "editor_a71905973c44",
	"批量编辑组件加载失败，请重试。": "editor_1cb29e089985",
	"正在加载批量编辑…":       "editor_9b1b7aa6c1a9",
	"名称":              "editor_d44e9b3d3b31",
	"地址":              "editor_37aac5b4e639",
	"所属位置":            "editor_48bb6e70be10",
	"图标":              "editor_0d720eeea264",
	"描述":              "editor_dc2ba467fc7a",
	"删除选中的 ":          "editor_f2c699b7c0d1",
	" 行？保存后生效。":       "editor_8ee2398fb0bc",
	"放弃进入表格后尚未同步的修改？已加入工作数据的更改仍保留。": "editor_d947b0ff2222",
	"选择图标":   "editor_6d8adad124d0",
	"选择内置图标": "editor_a6c683c26c02",
	"搜索图标名称（英文关键词）": "editor_e6dc85359701",
	"上一页":     "editor_c9b9ae7a6144",
	"下一页":     "editor_8a8542f69648",
	"正在加载图标…": "editor_3bfb3f462ab8",
	"无法加载图标，请确认登录状态后重试。": "editor_afee87409cb8",
	"没有找到图标，请尝试其他英文关键词。": "editor_be0f406a4949",
	"选择图标 ":        "editor_aa3385d47766",
	"预览不可用":        "editor_a1c3ce8944f8",
	"重试":           "editor_b8784c8dd563",
	"管理导航 · Flare": "editor_733820a1332b",
	"← 返回首页":       "editor_4cd7e6499ebe",
	"管理导航":         "editor_b926cb3f0d53",
	"更多":           "editor_38844b135cf7",
	"导入 / 导出":      "editor_a173733f1faa",
	"批量编辑":         "editor_f54195cf117f",
	"恢复编辑草稿":       "editor_567b0566ef26",
	"使用帮助":         "editor_df01e14ded47",
	"搜索名称、地址和描述":   "editor_0a27258b3bc7",
	"管理分类":         "editor_ee46fd8708d9",
	"＋ 新增":         "editor_07a04de199fd",
	"正在加载…":        "editor_21bd738e0d71",
	"条目已移除":        "editor_92ca39e5f595",
	"撤销":           "editor_926a50b98ece",
	"粘贴或连续修改多行；更改仍需通过底部保存。应用与各分类内的顺序按行顺序保存。": "editor_fe3804c65af4",
	"返回列表":         "editor_e9d5ca6c1406",
	"新增行":          "editor_6e3cf65b1619",
	"编辑单元格":        "editor_6e7b06a9f6d0",
	"删除选中行":        "editor_42483e2874d3",
	"放弃本次表格修改":     "editor_d6192bdf815c",
	"批量编辑表格，可横向滚动": "editor_f571a668e2c2",
	"正在加载数据…":      "editor_1fc40c59a932",
	"导出待保存数据":      "editor_d6ce9d76cb0c",
	"重新载入":         "editor_35588dcb9be3",
	"放弃更改":         "editor_ef804eda908b",
	"保存更改":         "editor_ecf0ea01192d",
	"完成后，点击页面底部“保存更改”生效。": "editor_dba29038d864",
	"放在":        "editor_4bfe3a83fe41",
	"更多选项":      "editor_c611656faf0c",
	"图标名称或图片地址": "editor_ac9d63b2362c",
	"图标预览":      "editor_91c70b9d5f6d",
	"完成":        "editor_c0b3fbff51cc",
	"改名、排序和迁移会加入待保存内容。": "editor_6bb72fea04f1",
	"新增分类": "editor_351398e236a6",
	"添加":   "editor_7a8a11ead507",
	"编辑器需要 JavaScript。导航首页仍可正常使用。": "editor_8c36e68edbef",
	"找到 {0} 条，共 {1} 条":             "editor_b014268a1392",
	"共 {0} 条":                      "editor_c1d545326622",
	"共 {0} 行，已选择 {1} 行":            "editor_6e915b4c1f58",
	"第 {0} 行：{1}":                  "editor_8963ba7207de",
	"共 {0} 行":                      "editor_1503d284b3f0",
	"共 {0} 个图标，显示 {1}—{2}":         "editor_ba1c12123b4a",
	"草稿文件不能超过 64 MiB":              "editor_7a193c332aa7",
	"无法导出草稿：":                      "editor_647aa5f48012",
	"发现服务端恢复副本，请先检查恢复目录及数据一致性，再继续编辑：": "editor_bc126284168c",
}

func editorLocale() string {
	options, err := data.GetAllSettingsOptions()
	if err == nil && options.Locale == "en" {
		return "en"
	}
	return "zh"
}
func editorText(locale, source string) string {
	if key, ok := editorMessages[source]; ok {
		return i18n.T(locale, key)
	}
	return source
}
func editorMessageJSON(locale string) string {
	messages := map[string]string{}
	for source := range editorMessages {
		messages[source] = editorText(locale, source)
	}
	raw, err := json.Marshal(messages)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
