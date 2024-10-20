# CloudWeGo-Kitex

[English](README.md) | [中文](README_cn.md) | Português Brasil

[![Release](https://img.shields.io/github/v/release/cloudwego/kitex)](https://github.com/cloudwego/kitex/releases)
[![WebSite](https://img.shields.io/website?up_message=cloudwego&url=https%3A%2F%2Fwww.cloudwego.io%2F)](https://www.cloudwego.io/)
[![License](https://img.shields.io/github/license/cloudwego/kitex)](https://github.com/cloudwego/kitex/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/cloudwego/kitex)](https://goreportcard.com/report/github.com/cloudwego/kitex)
[![OpenIssue](https://img.shields.io/github/issues/cloudwego/kitex)](https://github.com/cloudwego/kitex/issues)
[![ClosedIssue](https://img.shields.io/github/issues-closed/cloudwego/kitex)](https://github.com/cloudwego/kitex/issues?q=is%3Aissue+is%3Aclosed)
![Stars](https://img.shields.io/github/stars/cloudwego/kitex)
![Forks](https://img.shields.io/github/forks/cloudwego/kitex)

Kitex [kaɪt'eks] é um framework Go RPC de **alto desempenho** e **forte extensibilidade** que ajuda desenvolvedores a construir microsserviços. Se o desempenho e a extensibilidade são as principais preocupações ao desenvolver microserviços, Kitex pode ser uma boa escolha.

## Funcionalidades básicas

- **Alto desempenho**

O kitex integra o [Netpoll](https://github.com/cloudwego/netpoll), uma biblioteca de rede de alto desempenho, que oferece uma vantagem de desempenho significativa em relação ao [go net](https://pkg.go.dev/net).

- **Extensibilidade**

O Kitex fornece muitas interfaces com implementação padrão para os usuários personalizarem. Você pode estendê-las ou injetá-las no Kitex para atender às suas necessidades (consulte a seção de extensão do framework abaixo).

- **Protocolo multi-mensagem**

O Kitex foi projetado para ser extensível para suportar múltiplos protocolos de mensagens RPC. A versão inicial contém suporte para **Thrift**, **Kitex Protobuf** e **gRPC**, no qual o Kitex Protobuf é um protocolo de mensagens Protobuf personalizado do Kitex com um formato de protocolo similizar ao Thrift. O Kitex também suporta desenvolvedores estendendo seus próprios protocolos de mensagens.

- **Protocolo de transporte múltiplo**

Para governança de serviços, o Kitex suporta **TTHeader e HTTP2**. O TTHeader pode ser usado em conjunto com Thrift e Kitex Protobuf.

- **Tipo de mensagem múltipla**

Kitex suporta **PingPong**, **One-way**, and **Bidirectional Streaming**. Entre eles, One-way atualmente suporta apenas o protocolo Thrift.

- **Governança de serviços**

O Kitex integra módulos de governança de serviço, como registro de serviço, descoberta de serviço balanceamento de carga, disjuntor, limitação de taxa, nova tentativa, monitoramento, rastreamento, registro diagnóstico, etc. A maioria deles foi fornecida com extensões padrão, dando aos usuários a opção de integrá-los conforme desejado.

- **Geração de código**

O Kitex possui ferramentas integradas de geração de código que oferecem suporte à geração de código **Thrift**, **Protobuf** e scaffold.

## Documentação

- [**Começando**](https://www.cloudwego.io/docs/kitex/getting-started/)

- **Guia do usuário**

  - **Características básicas**
  
    Incluindo tipo de mensagem, protocolos suportados, invocação direta, pool de conexões, controle de tempo limite, nova tentativa de solicitação, balanceador de carga, disjuntor, limitação de taxa, controle de instrumentação, registro e HttpResolver. [[mais]](https://www.cloudwego.io/docs/kitex/tutorials/basic-feature/)
    
  - **Funcionalidades de governança**
  
    Suporte à descoberta de serviços, monitoramento, rastreamento e controle de acesso personalizado. [[mais]](https://www.cloudwego.io/docs/kitex/tutorials/service-governance/)
    
  - **Funcionalidades avançadas**
  
    Suportando Modo SDK de chamada Gebérica e Servidor. [[mais]](https://www.cloudwego.io/docs/kitex/tutorials/advanced-feature/)
    
  - **Geração de código**
  
    Incluindo ferramenta de geração de código e serviço combinado. [[mais]](https://www.cloudwego.io/docs/kitex/tutorials/code-gen/)
    
  - **Framework de extensão**
  
    Fornencimento de extensões de middleware, extensões de suíte, registro de serviço, descoberta de serviço, balanceador de carga personalizado, monitoramento, registro codec, módulo de transporte, pipeline de transporte, transmissão transparente de metedados, módulo de diagnóstico. [[mais]](https://www.cloudwego.io/docs/kitex/tutorials/framework-exten/)
  
- **Referência**

  - Para Protocolo de Transporte, Instruções de Exceção e Especificação de Versão, consulte a [doc](https://www.cloudwego.io/docs/kitex/reference/).

- **Boas práticas**
  - As melhores práticas do Kitex em produção, como desligamento gracioso, tratamento de erros, teste de integração. [mais](https://www.cloudwego.io/docs/kitex/best-practice/)

- **FAQ**

  - Consulte o [FAQ](https://www.cloudwego.io/docs/kitex/faq/).

## Performance

O benchmark de desempenho pode fornecer apenas uma referência limitada. Na produção, há muitos fatores que podem afetar o desempenho real.

Fornecemos o projeto [kitex-benchmark](https://github.com/cloudwego/kitex-benchmark) para rastrear e comparar o desempenho do Kitex e de outras estruturas sob diferentes condições para referência.

## Projetos relacionados

- [Netpoll](https://github.com/cloudwego/netpoll): Uma biblioteca de rede de alto desempenho.
- [kitex-contrib](https://github.com/kitex-contrib): Uma biblioteca de extensão parcial do Kitex, que os usuários podem integrar ao Kitex por meio de opções de acordo com suas necessidades.
- [kitex-examples](https://github.com/cloudwego/kitex-examples): Exemplos de Kitex apresentando vários recursos.
- [biz-demo](https://github.com/cloudwego/biz-demo): demonstrações de negócios usando Kitex.

## Blogs
- [Melhorando o desempenho na arquitetura de microsserviços com Kitex](https://www.cloudwego.io/blog/2024/01/29/enhancing-performance-in-microservice-architecture-with-kitex/)
- [CloudWeGo: Uma prática líder na criação de middleware nativo em nuvem empresarial!](https://www.cloudwego.io/blog/2023/06/15/cloudwego-a-leading-practice-for-building-enterprise-cloud-native-middleware/)
- [Kitex: Unificando práticas de código aberto para uma estrutura RPC de alto desempenho](https://www.cloudwego.io/blog/2022/09/30/kitex-unifying-open-source-practice-for-a-high-performance-rpc-framework/)
- [Otimização de desempenho no Kitex](https://www.cloudwego.io/blog/2021/09/23/performance-optimization-on-kitex/)
- [ByteDance Practice na biblioteca da rede Go](https://www.cloudwego.io/blog/2021/10/09/bytedance-practices-on-go-network-library/)
- [Introdução à prática do Kitex: Guia de teste de desempenho](https://www.cloudwego.io/blog/2021/11/24/getting-started-with-kitexs-practice-performance-testing-guide/)

## Contribuição

Guia do colaborador: [Contributing](https://github.com/cloudwego/kitex/blob/develop/CONTRIBUTING.md).

## Licença

O Kitex é distribuído sob a [Apache Licença, version 2.0](https://github.com/cloudwego/kitex/blob/develop/LICENSE). As licenças de dependências de terceiros do Kitex são explicadas [aqui](https://github.com/cloudwego/kitex/blob/develop/licenses).

## Comunidade
- Email: [conduct@cloudwego.io](conduct@cloudwego.io)
- Como se tornar um membro: [COMMUNITY MEMBERSHIP](https://github.com/cloudwego/community/blob/main/COMMUNITY_MEMBERSHIP.md)
- Issues: [Issues](https://github.com/cloudwego/kitex/issues)
- Discord: Junte-se à comunidade do discord [Discord Channel](https://discord.gg/jceZSE7DsW).
- Lark: escaneia o QR code abaixo com [Lark](https://www.larksuite.com/zh_cn/download) para se juntar ao nosso grupo de usuários.

    ![LarkGroup](images/lark_group.png)

## Paisagens

<p align="center">
<img src="https://landscape.cncf.io/images/cncf-landscape-horizontal-color.svg" width="150"/>&nbsp;&nbsp;<img src="https://www.cncf.io/wp-content/uploads/2023/04/cncf-main-site-logo.svg" width="200"/>
<br/><br/>
CloudWeGo enriches the <a href="https://landscape.cncf.io/">CNCF CLOUD NATIVE Landscape</a>.
</p>
