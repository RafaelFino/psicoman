// Package google integra o Psicoman ao Google: Calendar (síncrono, único Google
// no caminho de request) e Drive (fallback/DR, fora do caminho de request).
//
// Gmail está FORA do escopo desta arquitetura (D9b): a plataforma gera o PDF de
// cobrança e o envio é por WhatsApp, fora da plataforma. O domínio conhece a
// integração só por interfaces (porta de Calendar); a implementação concreta vive
// aqui, atrás de interface + fake para teste, e degrada sem panic quando o Google
// não está configurado (R8).
package google
