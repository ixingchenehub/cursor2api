package utils

import (
	"strings"

	"cursor2api/types"
)

// NormalizeToolName standardizes tool names by replacing underscores with hyphens
func NormalizeToolName(name string) string {
	return strings.ReplaceAll(name, "_", "-")
}

// FindToolByName finds a tool by name with fuzzy matching
// Returns nil if no tool is found
func FindToolByName(toolName string, tools []types.Tool) *types.Tool {
	// Direct match
	for i := range tools {
		if tools[i].Function.Name == toolName {
			return &tools[i]
		}
	}
	
	// Normalized match
	normalizedInput := NormalizeToolName(toolName)
	for i := range tools {
		if NormalizeToolName(tools[i].Function.Name) == normalizedInput {
			return &tools[i]
		}
	}
	
	return nil
}