package editor

import (
	"encoding/json"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/internal/i18n"
)

var editorMessages = map[string]string{
	"编辑器已升级，请先保留当前内容，再刷新页面后重新编辑。":     "editor.the_editor_was_upgraded_keep_your_pending_edits_then",
	"无法读取导航数据，请检查配置文件及日志。":            "editor.cannot_read_navigation_data_check_the_configuration_files_and",
	"请从 Flare 编辑器保存":                  "editor.save_from_the_flare_editor",
	"提交数据格式无效":                        "editor.invalid_submitted_document",
	"服务器数据已变化，当前修改未保存。请导出待保存数据或重新载入。": "editor.server_data_changed_your_edits_were_not_saved_export",
	"分类 ID 缺失或重复":                     "editor.category_id_is_missing_or_duplicated",
	"分类名称不能为空、重复或使用保留名称":              "editor.category_names_must_not_be_empty_duplicated_or_reserved",
	"应用不能引用书签分类":                      "editor.applications_cannot_reference_bookmark_categories",
	"书签引用的分类不存在，请选择有效位置":              "editor.the_bookmark_category_does_not_exist_choose_a_valid",
	"保存失败且恢复未完成，请检查服务端恢复副本。":          "editor.save_failed_and_recovery_is_incomplete_inspect_the_server",
	"保存失败，已恢复已替换文件；请检查服务端恢复副本。":       "editor.save_failed_replaced_files_were_restored_inspect_the_server",
	"保存失败，请检查服务端日志后重试。":               "editor.save_failed_check_the_server_logs_before_retrying",
	"搜索文字过长": "editor.search_text_is_too_long",
	"无效分页位置": "editor.invalid_page_offset",

	"应用区": "editor.applications",

	"未分类书签": "editor.uncategorized_bookmarks",

	"全部位置":  "editor.all_locations",
	"请选择位置": "editor.choose_a_location",
	"保存恢复未完成，请先导出草稿并检查服务端恢复副本，再重新载入。": "editor.save_recovery_is_incomplete_export_your_draft_inspect_server",
	"服务器数据已变化，请保留修改后重新载入。":            "editor.server_data_changed_keep_your_edits_before_reloading",
	"有未保存的更改": "editor.unsaved_changes",
	"所有更改已保存": "editor.all_changes_saved",

	"没有找到匹配条目":        "editor.no_matching_entries",
	"还没有条目，从新增或导入开始。": "editor.no_entries_yet_add_or_import_an_entry",
	"清除筛选":            "editor.clear_filters",
	"新增条目":            "editor.new_entry",
	"编辑":              "editor.edit",
	"删除":              "editor.delete",
	"编辑条目":            "editor.edit_entry",
	"放弃本次表单修改？":       "editor.discard_changes_in_this_form",
	"请填写名称":           "editor.enter_a_name",
	"请填写地址":           "editor.enter_an_address",
	"请选择有效位置":         "editor.choose_a_valid_location",

	"条目已移除，保存后生效；可撤销本次删除。": "editor.entry_removed_save_changes_to_apply_you_can_undo",
	"还没有书签分类。":             "editor.no_bookmark_categories_yet",

	"名称不能为空、重复或使用保留名称": "editor.the_name_must_not_be_empty_duplicated_or_reserved",

	"上移": "editor.move_up",
	"下移": "editor.move_down",

	"迁入位置":  "editor.move_to",
	"取消":    "editor.cancel",
	"迁移并删除": "editor.move_and_delete",
	"尚未添加的新分类名称将被放弃，继续？":  "editor.discard_the_new_category_name_that_has_not_been",
	"分类名称或 ID 无效":         "editor.invalid_category_name_or_id",
	"正在保存…":               "editor.saving",
	"登录可能已过期，请在新窗口登录后重试。": "editor.your_session_may_have_expired_sign_in_in_another",
	"保存失败，请重试。":           "editor.save_failed_please_retry",
	"保存响应不完整，请检查数据后重试。":   "editor.the_save_response_is_incomplete_check_the_data_before",
	"保存结果待确认：请求中断或响应无法读取，服务端可能已经保存。请先导出草稿，再检查服务端数据；重新保存仍会校验版本。 当前编辑内容已保留。": "editor.save_outcome_unknown_the_request_was_interrupted_or_its",

	"放弃全部未保存的更改？":                 "editor.discard_all_unsaved_changes",
	"已恢复加载时的数据；服务器已有更新，请重新载入。":    "editor.the_loaded_snapshot_was_restored_the_server_has_newer",
	"重新载入会放弃当前修改，请先导出需要保留的内容。继续？": "editor.reloading_discards_your_edits_export_anything_you_need_first",
	"不是有效的编辑器草稿":                  "editor.not_a_valid_editor_draft",
	"分类格式无效":                      "editor.invalid_category_format",
	"条目格式无效":                      "editor.invalid_entry_format",
	"以草稿替换当前待保存内容？保存时仍会检查本页加载的服务器版本。": "editor.replace_pending_edits_with_this_draft_saving_still_checks",
	"草稿已恢复到待保存内容，请检查后保存。":             "editor.draft_restored_to_pending_edits_review_it_before_saving",

	"更改尚未保存，确定离开？":         "editor.you_have_unsaved_changes_leave_this_page",
	"无法读取导航数据，请重新登录或检查配置。": "editor.cannot_load_navigation_data_sign_in_again_or_check",
	"加载失败":            "editor.loading_failed",
	"条目不存在":           "editor.entry_does_not_exist",
	"请填写名称或删除空行":      "editor.enter_a_name_or_delete_the_empty_row",
	"请填写地址或删除空行":      "editor.enter_an_address_or_delete_the_empty_row",
	"批量编辑组件加载失败，请重试。": "editor.cannot_load_batch_editing_please_retry",
	"正在加载批量编辑…":       "editor.loading_batch_editing",
	"名称":              "editor.name",
	"地址":              "editor.address",
	"所属位置":            "editor.location",
	"图标":              "editor.icon",
	"描述":              "editor.description",

	"放弃进入表格后尚未同步的修改？已加入工作数据的更改仍保留。": "editor.discard_table_edits_not_yet_synchronized_changes_already_in",
	"选择图标":   "editor.choose_icon",
	"选择内置图标": "editor.choose_a_built_in_icon",
	"搜索图标名称（英文关键词）": "editor.search_icon_names_english_keywords",
	"上一页":     "editor.previous",
	"下一页":     "editor.next",
	"正在加载图标…": "editor.loading_icons",
	"无法加载图标，请确认登录状态后重试。": "editor.cannot_load_icons_check_your_session_and_retry",
	"没有找到图标，请尝试其他英文关键词。": "editor.no_icons_found_try_another_english_keyword",

	"预览不可用":        "editor.preview_unavailable",
	"重试":           "editor.retry",
	"管理导航 · Flare": "editor.manage_navigation_flare",
	"← 返回首页":       "editor.home",
	"管理导航":         "editor.manage_navigation",
	"更多":           "editor.more",
	"导入 / 导出":      "editor.import_export",
	"批量编辑":         "editor.batch_editing",
	"恢复编辑草稿":       "editor.restore_editor_draft",
	"使用帮助":         "editor.help",
	"搜索名称、地址和描述":   "editor.search_names_addresses_and_descriptions",
	"管理分类":         "editor.manage_categories",
	"＋ 新增":         "editor.new",
	"正在加载…":        "editor.loading",
	"条目已移除":        "editor.entry_removed",
	"撤销":           "editor.undo",
	"粘贴或连续修改多行；更改仍需通过底部保存。应用与各分类内的顺序按行顺序保存。": "editor.paste_or_edit_multiple_rows_use_save_changes_at",
	"返回列表":         "editor.back_to_list",
	"新增行":          "editor.add_row",
	"编辑单元格":        "editor.edit_cell",
	"删除选中行":        "editor.delete_selected_rows",
	"放弃本次表格修改":     "editor.reset_table_changes",
	"批量编辑表格，可横向滚动": "editor.batch_editor_table_scroll_horizontally",
	"正在加载数据…":      "editor.loading_data",
	"导出待保存数据":      "editor.export_pending_data",
	"重新载入":         "editor.reload",
	"放弃更改":         "editor.discard_changes",
	"保存更改":         "editor.save_changes",
	"完成后，点击页面底部“保存更改”生效。": "editor.after_completing_this_form_use_save_changes_at_the",
	"放在":        "editor.place_in",
	"更多选项":      "editor.more_options",
	"图标名称或图片地址": "editor.icon_name_or_image_address",
	"图标预览":      "editor.icon_preview",
	"完成":        "editor.done",
	"改名、排序和迁移会加入待保存内容。": "editor.renaming_reordering_and_moving_update_pending_edits",
	"新增分类": "editor.new_category",
	"添加":   "editor.add",
	"编辑器需要 JavaScript。导航首页仍可正常使用。": "editor.the_editor_requires_javascript_the_navigation_homepage_remains_available",

	"草稿文件不能超过 64 MiB": "editor.draft_files_must_not_exceed_64_mib",
}

func editorLocale() string {
	options, err := data.GetAllSettingsOptions()
	if err == nil {
		return i18n.Normalize(options.Locale)
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
	for _, key := range editorMessages {
		messages[key] = i18n.T(locale, key)
	}
	for _, key := range editorCompositeMessages {
		messages[key] = i18n.T(locale, key)
	}
	raw, err := json.Marshal(messages)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

// Composite messages keep parameters and punctuation in a single translation.
var editorCompositeMessages = []string{
	"editor.default_category",
	"editor.missing_category",
	"editor.entry_count",
	"editor.move_success",
	"editor.category_name_label",
	"editor.bookmark_count",
	"editor.delete_empty_category",
	"editor.category_migration_hint",
	"editor.error_draft_preserved",
	"editor.draft_export_error",
	"editor.draft_restore_error",
	"editor.recovery_paths_warning",
	"editor.confirm_delete_selected_rows",
	"editor.choose_named_icon",
	"editor.search_result_count",
	"editor.row_selection_count",
	"editor.row_error",
	"editor.row_count",
	"editor.icon_result_count",
}
