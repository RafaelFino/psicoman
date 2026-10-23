// Package app abriga os serviços de negócio enxutos do Psicoman.
//
// Os serviços apenas ORQUESTRAM: coordenam os agregados de domínio e as portas
// (repositórios, Calendar, IA), sem concentrar regra que pertence ao domínio
// (R4). Dependem do domínio e das interfaces de porta — nunca do PocketBase nem
// do adapter platform/pb (que implementa as portas).
package app
