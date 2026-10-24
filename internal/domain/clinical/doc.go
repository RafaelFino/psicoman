// Package clinical é o bounded context clínico: notas de sessão, nota livre,
// anamnese, resumo de IA e índice de tags. Os campos clínicos sensíveis usam o
// valor de domínio SealedText (cifra por campo, D6); as tags ficam em claro para
// habilitar a busca. Ver MER canônico (D4b).
package clinical
