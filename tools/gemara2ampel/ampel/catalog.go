package ampel

import "github.com/gemaraproj/go-gemara"

// CatalogEnrichment contains enriched information from a catalog lookup.
type CatalogEnrichment struct {
	// Control is the control that contains the requirement
	Control *gemara.Control

	// Requirement is the specific assessment requirement
	Requirement *gemara.AssessmentRequirement

	// Family is the control group (formerly family)
	Family *gemara.Group
}

// lookupRequirement finds a requirement in the catalog by its ID.
// Returns the control containing the requirement, the requirement itself,
// and the control family, or nil if not found.
func lookupRequirement(catalog *gemara.ControlCatalog, requirementID string) *CatalogEnrichment {
	if catalog == nil || requirementID == "" {
		return nil
	}

	// Search through all controls for the requirement
	for i := range catalog.Controls {
		control := &catalog.Controls[i]
		for j := range control.AssessmentRequirements {
			req := &control.AssessmentRequirements[j]
			if req.Id == requirementID {
				// Find the group for this control
				var family *gemara.Group
				for k := range catalog.Groups {
					if catalog.Groups[k].Id == control.Group {
						family = &catalog.Groups[k]
						break
					}
				}

				return &CatalogEnrichment{
					Control:     control,
					Requirement: req,
					Family:      family,
				}
			}
		}
	}

	return nil
}

// enrichTenetTitle creates an enriched tenet title using catalog data.
// Prefers the assessment requirement text as the most specific description.
// Falls back to the provided default title if catalog data is not available.
func enrichTenetTitle(enrichment *CatalogEnrichment, defaultTitle string) string {
	if enrichment == nil {
		return defaultTitle
	}

	// Prefer the assessment requirement text as the most specific description
	if enrichment.Requirement != nil && enrichment.Requirement.Text != "" {
		return enrichment.Requirement.Text
	}

	// Fall back to control title
	if enrichment.Control != nil && enrichment.Control.Title != "" {
		return enrichment.Control.Title
	}

	return defaultTitle
}

// createControlReference creates a Control reference for the policy metadata.
// This can be added to the Meta.Controls field to track which controls
// are verified by the policy.
func createControlReference(enrichment *CatalogEnrichment) *Control {
	if enrichment == nil || enrichment.Control == nil {
		return nil
	}

	control := &Control{
		Id:    enrichment.Control.Id,
		Title: enrichment.Control.Title,
	}

	// Add family information if available
	if enrichment.Family != nil {
		// Use family title as framework name
		control.Framework = enrichment.Family.Title
		// Use family ID as class
		control.Class = enrichment.Family.Id
	}

	return control
}

// collectControlReferences collects all unique control references from tenets
// that were enriched with catalog data. This is used to populate the policy-level
// Meta.Controls field.
func collectControlReferences(enrichments []*CatalogEnrichment) []*Control {
	// Use a map to deduplicate controls
	controlsMap := make(map[string]*Control)

	for _, enrichment := range enrichments {
		if enrichment == nil || enrichment.Control == nil {
			continue
		}

		controlID := enrichment.Control.Id
		if _, exists := controlsMap[controlID]; !exists {
			controlsMap[controlID] = createControlReference(enrichment)
		}
	}

	// Convert map to slice
	controls := make([]*Control, 0, len(controlsMap))
	for _, control := range controlsMap {
		controls = append(controls, control)
	}

	return controls
}
