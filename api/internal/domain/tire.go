package domain

import (
	"errors"
	"math"
)

type Classification struct {
	Status         string `json:"status"`
	Level          int    `json:"level"`
	Recommendation string `json:"recommendation"`
}

func Classify(tread float64, damage bool) (Classification, error) {
	return ClassifyWithThresholds(tread, damage, 5, 3)
}
func ClassifyWithThresholds(tread float64, damage bool, warning, critical float64) (Classification, error) {
	if math.IsNaN(warning) || math.IsNaN(critical) || math.IsInf(warning, 0) || math.IsInf(critical, 0) || critical <= 0 || warning <= critical || warning > 40 {
		return Classification{}, errors.New("limites de sulco inválidos")
	}
	if math.IsNaN(tread) || math.IsInf(tread, 0) || tread < 0 || tread > 40 {
		return Classification{}, errors.New("sulco inválido")
	}
	if damage {
		return Classification{"critico", 3, "Retirar de operação e avaliar a carcaça imediatamente."}, nil
	}
	if tread < critical {
		return Classification{"critico", 2, "Intervenção imediata: retirar de operação para avaliação."}, nil
	}
	if tread < warning {
		return Classification{"alerta", 1, "Planejar troca ou avaliação para recapagem."}, nil
	}
	return Classification{"ok", 0, "Manter inspeções e calibragem periódicas."}, nil
}
