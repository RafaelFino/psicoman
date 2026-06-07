# Sistema de gestão de atendimentos de psicologia

## Requisitos do sistema
- Precisamos de um sistema que permita o cadastro de pacientes, para controlar todos os atendimentos com a visão de um psicólogo.
- Esse sistema deve oferecer também uma área web separada, para que os pacientes possam acessar e gerenciar seus atendimentos além de ter também um formulário para que os pacientes possam wse cadastrar e fazer a parte da anaminése.
- O sistema em backend também deve ter um GED simples, para armazenar arquivos de atendimentos, como laudos, notas fiscais e documentos gerados pelo sistema e enviados pelos pacientes.
- O sistema deve oferecer uma interface web e responsiva (funcionando tanto em celulares como em navegaores desktop) para a operação do psicologo, para que este consiga anotar os atendimentos, gerenciar os pacientes e gerenciar os atendimentos.
- O sistema deve se integrar com a agenda do Google e por ali agendar os atendimentos.
- O sistema deve também, integrado a api do google, reservar espaços no google meeting para os atendimentos.
- Os atendimentos podem ser presenciais ou online, mas todos devem ter o link para o meetings mesmo assim.
- O sistema deve ter a capacidade de mostrar as agendas dos atendimentos, tanto na visão do psicologo como para o paciente. O paciente só vê os próprios atendimentos e os horários livres para novos agendamentos.
- Deve haver uma área para que os pacientes possam acessar o GED e baixar os documentos gerados pelo sistema e enviados pelo psicologo.
- Deve haver uma área para que os pacientes possam agendar consultas com o psicologo. 
- Devem haver regras para desmarcar atendimentos, cancelar atendimentos e reagendar atendimentos. Precisamos de uma área para o psicologo cancelar atendimentos e reagendar atendimentos e uma forma para o psicologo conseguir configurar essas regras.
- O sistema deve ter temas claros, interface moderna e limpa, considerando que os usuários tem pouca afinidade com tecnologia
- Na área onde o psicologo preenche os relatórios, devemos ter um editor de textos simples, para escrever os relatórios.
- O pscilogo deve ter a capacidade de extrair relatório, laudos e uma visão completa do paciente, dos agendamentos.
- Precisamos de relatórios mensais para a apuração de quantas e quais foram as consultas de cada paciênte, pensando que devemos emitir notas fiscais mensalmente
- É preciso uma parte para gestão de custos e pagamentos a receber, para que o psciologo consiga controlar os custos mensais e quais pagamentos tem a receber e quais já recebeu, relatórios consolidados aqui são importantes para termos os fechamentos mensais
- o psicologo será autenticado via pangolin e será o admin desse sistema, mas os pacientes devem se autenticar usando a conta do gmail como social auth, devemos lidar com um jwt nessa parte e deixar bem separado oq é área do paciente e oq é do psicilogo. Podemos ter mais de um admin aqui, mas apenas um psicologo, dentro da solução podemos criar esses perfis (admin e psicologo), os pacientes tem a capacidade de criarem suas contas sozinhos

## Stack de tecnologia
- Vamos criar um serviço com backend em golang, frontend em react e banco de dados em sqlite
- Não precisamos nos preocupar com a questão de autenticação, temos uma arch de infra que garante isso. Temos um servidor local onde a aplicação irá rodar e como proxy para receber requisições da internet, temos um servidor em uma cloud (oci-gateway) rodando uma solução chamada pangolin, toda a gestão de usuários acontece por lá e os usuários chegam em um header HTTP de todas as requisições para o nosso serviço.
- Cada banco de dados SQLite será de um usuário diferente, portanto de acordo com ousuário que recebermos nesse header do pangolin, vamos apontar para um sqlite correspondente
- esse serviço em golang, que vamos chamar de server, deve hospedar o HTML e também ser o backend, estamos falando de apenas um serviço em golang apenas.
- vamos usar o framework gin para criar o servidor em golang.
- vamos usar o framework react para criar o frontend.
- Esse sistema deve ser auto contido em um serviço apenas, caso tenhamos cenários de mais psicologos usando esse sistema, teremos outra instância do serviço rodando em outro servidor ou container docker de forma totalmente apartada
- O sistema deve gerar logs de tudo, em um formato jason, em arquivo e ter rotação de logs diários. Todos os acessos devem ser logados.
- O sistema deve ter testes integrados e testes unitários para todas as partes
- Vamos usar uma estrutura simples de camadas (domain, service, storage, web, cmd) para a estrutura em go
- Considere que vamos rodar esse sistema inteiro em um docker-compose, mas o banco sqlite e os logs devem ficar fora do compose, devem ser mapeados de uma pasta no host
