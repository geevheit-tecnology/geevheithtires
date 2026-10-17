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
	if math.IsNaN(tread) || math.IsInf(tread, 0) || tread < 0 || tread > 40 {
		return Classification{}, errors.New("sulco inválido")
	}
	if damage {
		return Classification{"critico", 3, "Retirar de operação e avaliar a carcaça imediatamente."}, nil
	}
	if tread < 3 {
		return Classification{"critico", 2, "Intervenção imediata: retirar de operação para avaliação."}, nil
	}
	if tread < 5 {
		return Classification{"alerta", 1, "Planejar troca ou avaliação para recapagem."}, nil
	}
	return Classification{"ok", 0, "Manter inspeções e calibragem periódicas."}, nil
}
