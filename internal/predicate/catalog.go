package predicate

// The catalog is the harmless projection of the registry the browser is allowed
// to see: display labels, categories, and the legal feature→operator→operand
// relationships that let the UI make invalid predicates unselectable. It
// deliberately contains NO SQL — no table names, columns, expressions, or schema
// (design §8). The server re-validates every ID on the way back in, so a doctored
// catalog buys an attacker nothing.

// CatFeature is a selectable left-hand feature plus the operators it allows.
type CatFeature struct {
	ID          FeatureID    `json:"id"`
	Label       string       `json:"label"`
	Category    string       `json:"category"`
	Sortable    bool         `json:"sortable"`
	OperatorIDs []OperatorID `json:"operatorIds"`
}

// CatOperator / CatOperand are label-only lookups keyed by ID.
type (
	CatOperator struct {
		ID    OperatorID `json:"id"`
		Label string     `json:"label"`
	}
	CatOperand struct {
		ID       OperandID `json:"id"`
		Label    string    `json:"label"`
		Category string    `json:"category"`
	}
)

// Catalog is the full browser-facing scanner catalog.
type Catalog struct {
	Version   int           `json:"version"`
	Features  []CatFeature  `json:"features"`
	Operators []CatOperator `json:"operators"`
	Operands  []CatOperand  `json:"operands"`
	// Compatibility[featureID][operatorID] = allowed operand IDs. This is what the
	// UI uses to cascade: pick a feature → valid operators → valid right operands.
	Compatibility map[FeatureID]map[OperatorID][]OperandID `json:"compatibility"`
	// SortFeatures is the subset of features approved as ORDER BY targets.
	SortFeatures []FeatureID `json:"sortFeatures"`
}

// BuildCatalog projects the registry into the browser catalog. Operand ordering
// within each compatibility list follows registry order for deterministic output.
func BuildCatalog() Catalog {
	operandOrder := make(map[OperandID]int, len(Operands))
	for i, o := range Operands {
		operandOrder[o.ID] = i
	}

	c := Catalog{Version: 1, Compatibility: map[FeatureID]map[OperatorID][]OperandID{}}

	for _, f := range Features {
		ops := Compatibility[f.ID]
		if len(ops) == 0 {
			continue // a feature with no legal comparisons is not offered
		}
		var opIDs []OperatorID
		byOp := map[OperatorID][]OperandID{}
		for _, op := range Operators { // registry order, stable
			set, ok := ops[op.ID]
			if !ok {
				continue
			}
			opIDs = append(opIDs, op.ID)
			operands := make([]OperandID, 0, len(set))
			for id := range set {
				operands = append(operands, id)
			}
			sortByOrder(operands, operandOrder)
			byOp[op.ID] = operands
		}
		c.Features = append(c.Features, CatFeature{
			ID: f.ID, Label: f.Label, Category: f.Category, Sortable: f.Sortable, OperatorIDs: opIDs,
		})
		c.Compatibility[f.ID] = byOp
		if f.Sortable {
			c.SortFeatures = append(c.SortFeatures, f.ID)
		}
	}
	for _, op := range Operators {
		c.Operators = append(c.Operators, CatOperator{ID: op.ID, Label: op.Label})
	}
	for _, o := range Operands {
		c.Operands = append(c.Operands, CatOperand{ID: o.ID, Label: o.Label, Category: o.Category})
	}
	return c
}

func sortByOrder(ids []OperandID, order map[OperandID]int) {
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && order[ids[j]] < order[ids[j-1]]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
}
