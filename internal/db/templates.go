package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const templateTimestampLayout = "2006-01-02 15:04:05"

type RuleTemplate struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type TemplateRule struct {
	ID                int64  `json:"id"`
	TemplateID        int64  `json:"template_id"`
	Position          int    `json:"position"`
	Name              string `json:"name"`
	Enabled           bool   `json:"enabled"`
	Precedence        bool   `json:"precedence"`
	Action            string `json:"action"`
	Duration          string `json:"duration"`
	OperatorType      string `json:"operator_type"`
	OperatorSensitive bool   `json:"operator_sensitive"`
	OperatorOperand   string `json:"operator_operand"`
	OperatorData      string `json:"operator_data"`
	OperatorJSON      string `json:"-"`
	Description       string `json:"description"`
	Nolog             bool   `json:"nolog"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

type TemplateAttachment struct {
	ID         int64  `json:"id"`
	TemplateID int64  `json:"template_id"`
	TargetType string `json:"target_type"`
	TargetRef  string `json:"target_ref"`
	Priority   int    `json:"priority"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type NodeTemplateSync struct {
	Node      string `json:"node"`
	Pending   bool   `json:"pending"`
	Error     string `json:"error"`
	UpdatedAt string `json:"updated_at"`
}

func nowTemplateTimestamp() string {
	return time.Now().Format(templateTimestampLayout)
}

func NormalizeTag(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}

	var builder strings.Builder
	lastDash := false
	for _, ch := range value {
		switch {
		case ch >= 'a' && ch <= 'z':
			builder.WriteRune(ch)
			lastDash = false
		case ch >= '0' && ch <= '9':
			builder.WriteRune(ch)
			lastDash = false
		default:
			if builder.Len() == 0 || lastDash {
				continue
			}
			builder.WriteByte('-')
			lastDash = true
		}
	}

	return strings.Trim(builder.String(), "-")
}

func normalizeTags(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		tag := NormalizeTag(value)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, tag)
	}
	return result
}

func normalizeAttachmentTarget(targetType, targetRef string) string {
	if targetType == "tag" {
		return NormalizeTag(targetRef)
	}
	return strings.TrimSpace(targetRef)
}

func (d *Database) CreateRuleTemplate(tpl *RuleTemplate) (*RuleTemplate, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := nowTemplateTimestamp()
	result, err := d.db.Exec(
		"INSERT INTO rule_templates (name, description, created_at, updated_at) VALUES (?, ?, ?, ?)",
		strings.TrimSpace(tpl.Name), strings.TrimSpace(tpl.Description), now, now,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &RuleTemplate{
		ID:          id,
		Name:        strings.TrimSpace(tpl.Name),
		Description: strings.TrimSpace(tpl.Description),
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (d *Database) GetRuleTemplates() ([]RuleTemplate, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query("SELECT id, name, description, created_at, updated_at FROM rule_templates ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []RuleTemplate
	for rows.Next() {
		var tpl RuleTemplate
		if err := rows.Scan(&tpl.ID, &tpl.Name, &tpl.Description, &tpl.CreatedAt, &tpl.UpdatedAt); err != nil {
			return nil, err
		}
		templates = append(templates, tpl)
	}

	return templates, nil
}

func (d *Database) GetRuleTemplate(id int64) (*RuleTemplate, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var tpl RuleTemplate
	err := d.db.QueryRow("SELECT id, name, description, created_at, updated_at FROM rule_templates WHERE id = ?", id).
		Scan(&tpl.ID, &tpl.Name, &tpl.Description, &tpl.CreatedAt, &tpl.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &tpl, nil
}

func (d *Database) UpdateRuleTemplate(tpl *RuleTemplate) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(
		"UPDATE rule_templates SET name = ?, description = ?, updated_at = ? WHERE id = ?",
		strings.TrimSpace(tpl.Name), strings.TrimSpace(tpl.Description), nowTemplateTimestamp(), tpl.ID,
	)
	return err
}

func (d *Database) DeleteRuleTemplate(id int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}

	if _, err := tx.Exec("DELETE FROM template_attachments WHERE template_id = ?", id); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.Exec("DELETE FROM template_rules WHERE template_id = ?", id); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.Exec("DELETE FROM rule_templates WHERE id = ?", id); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (d *Database) CreateTemplateRule(rule *TemplateRule) (*TemplateRule, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := nowTemplateTimestamp()
	position := rule.Position
	if position <= 0 {
		if err := d.db.QueryRow("SELECT COALESCE(MAX(position), 0) + 1 FROM template_rules WHERE template_id = ?", rule.TemplateID).Scan(&position); err != nil {
			return nil, err
		}
	}

	result, err := d.db.Exec(
		`INSERT INTO template_rules
			(template_id, position, name, enabled, precedence, action, duration, operator_type, operator_sensitive, operator_operand, operator_data, operator_json, description, nolog, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.TemplateID, position, strings.TrimSpace(rule.Name), rule.Enabled, rule.Precedence, rule.Action, rule.Duration,
		rule.OperatorType, rule.OperatorSensitive, rule.OperatorOperand, rule.OperatorData, rule.OperatorJSON, strings.TrimSpace(rule.Description), rule.Nolog, now, now,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	created := *rule
	created.ID = id
	created.Position = position
	created.Name = strings.TrimSpace(rule.Name)
	created.Description = strings.TrimSpace(rule.Description)
	created.CreatedAt = now
	created.UpdatedAt = now
	return &created, nil
}

func (d *Database) GetTemplateRules(templateID int64) ([]TemplateRule, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(
		`SELECT id, template_id, position, name, enabled, precedence, action, duration, operator_type, operator_sensitive, operator_operand, operator_data, operator_json, description, nolog, created_at, updated_at
		FROM template_rules WHERE template_id = ? ORDER BY position, id`,
		templateID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []TemplateRule
	for rows.Next() {
		var rule TemplateRule
		if err := rows.Scan(&rule.ID, &rule.TemplateID, &rule.Position, &rule.Name, &rule.Enabled, &rule.Precedence, &rule.Action, &rule.Duration, &rule.OperatorType, &rule.OperatorSensitive, &rule.OperatorOperand, &rule.OperatorData, &rule.OperatorJSON, &rule.Description, &rule.Nolog, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

func (d *Database) GetTemplateRule(templateID, ruleID int64) (*TemplateRule, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var rule TemplateRule
	err := d.db.QueryRow(
		`SELECT id, template_id, position, name, enabled, precedence, action, duration, operator_type, operator_sensitive, operator_operand, operator_data, operator_json, description, nolog, created_at, updated_at
		FROM template_rules WHERE template_id = ? AND id = ?`,
		templateID, ruleID,
	).Scan(&rule.ID, &rule.TemplateID, &rule.Position, &rule.Name, &rule.Enabled, &rule.Precedence, &rule.Action, &rule.Duration, &rule.OperatorType, &rule.OperatorSensitive, &rule.OperatorOperand, &rule.OperatorData, &rule.OperatorJSON, &rule.Description, &rule.Nolog, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &rule, nil
}

func (d *Database) UpdateTemplateRule(rule *TemplateRule) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(
		`UPDATE template_rules
		SET position = ?, name = ?, enabled = ?, precedence = ?, action = ?, duration = ?, operator_type = ?, operator_sensitive = ?, operator_operand = ?, operator_data = ?, operator_json = ?, description = ?, nolog = ?, updated_at = ?
		WHERE template_id = ? AND id = ?`,
		rule.Position, strings.TrimSpace(rule.Name), rule.Enabled, rule.Precedence, rule.Action, rule.Duration,
		rule.OperatorType, rule.OperatorSensitive, rule.OperatorOperand, rule.OperatorData, rule.OperatorJSON, strings.TrimSpace(rule.Description), rule.Nolog, nowTemplateTimestamp(), rule.TemplateID, rule.ID,
	)
	return err
}

func (d *Database) DeleteTemplateRule(templateID, ruleID int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("DELETE FROM template_rules WHERE template_id = ? AND id = ?", templateID, ruleID)
	return err
}

func (d *Database) CreateTemplateAttachment(attachment *TemplateAttachment) (*TemplateAttachment, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := nowTemplateTimestamp()
	targetRef := normalizeAttachmentTarget(attachment.TargetType, attachment.TargetRef)
	result, err := d.db.Exec(
		"INSERT INTO template_attachments (template_id, target_type, target_ref, priority, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		attachment.TemplateID, strings.TrimSpace(attachment.TargetType), targetRef, attachment.Priority, now, now,
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	created := *attachment
	created.ID = id
	created.TargetType = strings.TrimSpace(attachment.TargetType)
	created.TargetRef = targetRef
	created.CreatedAt = now
	created.UpdatedAt = now
	return &created, nil
}

func (d *Database) GetTemplateAttachments(templateID int64) ([]TemplateAttachment, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query("SELECT id, template_id, target_type, target_ref, priority, created_at, updated_at FROM template_attachments WHERE template_id = ? ORDER BY priority, id", templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []TemplateAttachment
	for rows.Next() {
		var attachment TemplateAttachment
		if err := rows.Scan(&attachment.ID, &attachment.TemplateID, &attachment.TargetType, &attachment.TargetRef, &attachment.Priority, &attachment.CreatedAt, &attachment.UpdatedAt); err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}

	return attachments, nil
}

func (d *Database) GetAllTemplateAttachments() ([]TemplateAttachment, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query("SELECT id, template_id, target_type, target_ref, priority, created_at, updated_at FROM template_attachments ORDER BY template_id, priority, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []TemplateAttachment
	for rows.Next() {
		var attachment TemplateAttachment
		if err := rows.Scan(&attachment.ID, &attachment.TemplateID, &attachment.TargetType, &attachment.TargetRef, &attachment.Priority, &attachment.CreatedAt, &attachment.UpdatedAt); err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}

	return attachments, nil
}

func (d *Database) GetTemplateAttachment(templateID, attachmentID int64) (*TemplateAttachment, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var attachment TemplateAttachment
	err := d.db.QueryRow("SELECT id, template_id, target_type, target_ref, priority, created_at, updated_at FROM template_attachments WHERE template_id = ? AND id = ?", templateID, attachmentID).
		Scan(&attachment.ID, &attachment.TemplateID, &attachment.TargetType, &attachment.TargetRef, &attachment.Priority, &attachment.CreatedAt, &attachment.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &attachment, nil
}

func (d *Database) UpdateTemplateAttachment(attachment *TemplateAttachment) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	targetRef := normalizeAttachmentTarget(attachment.TargetType, attachment.TargetRef)
	_, err := d.db.Exec(
		"UPDATE template_attachments SET target_type = ?, target_ref = ?, priority = ?, updated_at = ? WHERE template_id = ? AND id = ?",
		strings.TrimSpace(attachment.TargetType), targetRef, attachment.Priority, nowTemplateTimestamp(), attachment.TemplateID, attachment.ID,
	)
	return err
}

func (d *Database) DeleteTemplateAttachment(templateID, attachmentID int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("DELETE FROM template_attachments WHERE template_id = ? AND id = ?", templateID, attachmentID)
	return err
}

func (d *Database) ReplaceNodeTags(node string, tags []string) ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	normalized := normalizeTags(tags)
	tx, err := d.db.Begin()
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec("DELETE FROM node_tags WHERE node = ?", node); err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	for _, tag := range normalized {
		if _, err := tx.Exec("INSERT INTO node_tags (node, tag) VALUES (?, ?)", node, tag); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return normalized, nil
}

func (d *Database) GetNodeTags(node string) ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query("SELECT tag FROM node_tags WHERE node = ? ORDER BY tag", node)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

func (d *Database) GetAllNodeTags() (map[string][]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query("SELECT node, tag FROM node_tags ORDER BY node, tag")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string][]string{}
	for rows.Next() {
		var node, tag string
		if err := rows.Scan(&node, &tag); err != nil {
			return nil, err
		}
		result[node] = append(result[node], tag)
	}

	return result, nil
}

func (d *Database) SetNodeTemplateSync(node string, pending bool, errorText string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(
		`INSERT INTO node_template_sync (node, pending, error, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(node) DO UPDATE SET
			pending = excluded.pending,
			error = excluded.error,
			updated_at = excluded.updated_at`,
		node, pending, strings.TrimSpace(errorText), nowTemplateTimestamp(),
	)
	return err
}

func (d *Database) GetNodeTemplateSync(node string) (*NodeTemplateSync, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var sync NodeTemplateSync
	err := d.db.QueryRow("SELECT node, pending, error, updated_at FROM node_template_sync WHERE node = ?", node).
		Scan(&sync.Node, &sync.Pending, &sync.Error, &sync.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return &NodeTemplateSync{Node: node}, nil
		}
		return nil, err
	}

	return &sync, nil
}

func (d *Database) GetAllNodeTemplateSync() (map[string]NodeTemplateSync, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query("SELECT node, pending, error, updated_at FROM node_template_sync")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]NodeTemplateSync{}
	for rows.Next() {
		var sync NodeTemplateSync
		if err := rows.Scan(&sync.Node, &sync.Pending, &sync.Error, &sync.UpdatedAt); err != nil {
			return nil, err
		}
		result[sync.Node] = sync
	}

	return result, nil
}

func (d *Database) MustGetRuleTemplate(id int64) (*RuleTemplate, error) {
	tpl, err := d.GetRuleTemplate(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("template %d not found", id)
		}
		return nil, err
	}
	return tpl, nil
}
