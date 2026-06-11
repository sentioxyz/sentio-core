/* eslint-disable */
// @ts-nocheck

/**
 * This file is a generated Typescript file for GRPC Gateway, DO NOT MODIFY
 */


export enum ProtocolVersion {
  UnspecifiedProtocolVersion = "UnspecifiedProtocolVersion",
}

export enum TextEncoding {
  UnspecifiedTextEncoding = "UnspecifiedTextEncoding",
  UTF8 = "UTF8",
  UTF16 = "UTF16",
}

export enum PositionEncoding {
  UnspecifiedPositionEncoding = "UnspecifiedPositionEncoding",
  UTF8CodeUnitOffsetFromLineStart = "UTF8CodeUnitOffsetFromLineStart",
  UTF16CodeUnitOffsetFromLineStart = "UTF16CodeUnitOffsetFromLineStart",
  UTF32CodeUnitOffsetFromLineStart = "UTF32CodeUnitOffsetFromLineStart",
}

export enum SymbolRole {
  UnspecifiedSymbolRole = "UnspecifiedSymbolRole",
  Definition = "Definition",
  Import = "Import",
  WriteAccess = "WriteAccess",
  ReadAccess = "ReadAccess",
  Generated = "Generated",
  Test = "Test",
  ForwardDefinition = "ForwardDefinition",
}

export enum SyntaxKind {
  UnspecifiedSyntaxKind = "UnspecifiedSyntaxKind",
  Comment = "Comment",
  PunctuationDelimiter = "PunctuationDelimiter",
  PunctuationBracket = "PunctuationBracket",
  Keyword = "Keyword",
  IdentifierKeyword = "IdentifierKeyword",
  IdentifierOperator = "IdentifierOperator",
  Identifier = "Identifier",
  IdentifierBuiltin = "IdentifierBuiltin",
  IdentifierNull = "IdentifierNull",
  IdentifierConstant = "IdentifierConstant",
  IdentifierMutableGlobal = "IdentifierMutableGlobal",
  IdentifierParameter = "IdentifierParameter",
  IdentifierLocal = "IdentifierLocal",
  IdentifierShadowed = "IdentifierShadowed",
  IdentifierNamespace = "IdentifierNamespace",
  IdentifierModule = "IdentifierModule",
  IdentifierFunction = "IdentifierFunction",
  IdentifierFunctionDefinition = "IdentifierFunctionDefinition",
  IdentifierMacro = "IdentifierMacro",
  IdentifierMacroDefinition = "IdentifierMacroDefinition",
  IdentifierType = "IdentifierType",
  IdentifierBuiltinType = "IdentifierBuiltinType",
  IdentifierAttribute = "IdentifierAttribute",
  RegexEscape = "RegexEscape",
  RegexRepeated = "RegexRepeated",
  RegexWildcard = "RegexWildcard",
  RegexDelimiter = "RegexDelimiter",
  RegexJoin = "RegexJoin",
  StringLiteral = "StringLiteral",
  StringLiteralEscape = "StringLiteralEscape",
  StringLiteralSpecial = "StringLiteralSpecial",
  StringLiteralKey = "StringLiteralKey",
  CharacterLiteral = "CharacterLiteral",
  NumericLiteral = "NumericLiteral",
  BooleanLiteral = "BooleanLiteral",
  Tag = "Tag",
  TagAttribute = "TagAttribute",
  TagDelimiter = "TagDelimiter",
}

export enum Severity {
  UnspecifiedSeverity = "UnspecifiedSeverity",
  Error = "Error",
  Warning = "Warning",
  Information = "Information",
  Hint = "Hint",
}

export enum DiagnosticTag {
  UnspecifiedDiagnosticTag = "UnspecifiedDiagnosticTag",
  Unnecessary = "Unnecessary",
  Deprecated = "Deprecated",
}

export enum Language {
  UnspecifiedLanguage = "UnspecifiedLanguage",
  ABAP = "ABAP",
  Apex = "Apex",
  APL = "APL",
  Ada = "Ada",
  Agda = "Agda",
  AsciiDoc = "AsciiDoc",
  Assembly = "Assembly",
  Awk = "Awk",
  Bat = "Bat",
  BibTeX = "BibTeX",
  C = "C",
  COBOL = "COBOL",
  CPP = "CPP",
  CSS = "CSS",
  CSharp = "CSharp",
  Clojure = "Clojure",
  Coffeescript = "Coffeescript",
  CommonLisp = "CommonLisp",
  Coq = "Coq",
  CUDA = "CUDA",
  Dart = "Dart",
  Delphi = "Delphi",
  Diff = "Diff",
  Dockerfile = "Dockerfile",
  Dyalog = "Dyalog",
  Elixir = "Elixir",
  Erlang = "Erlang",
  FSharp = "FSharp",
  Fish = "Fish",
  Flow = "Flow",
  Fortran = "Fortran",
  Git_Commit = "Git_Commit",
  Git_Config = "Git_Config",
  Git_Rebase = "Git_Rebase",
  Go = "Go",
  GraphQL = "GraphQL",
  Groovy = "Groovy",
  HTML = "HTML",
  Hack = "Hack",
  Handlebars = "Handlebars",
  Haskell = "Haskell",
  Idris = "Idris",
  Ini = "Ini",
  J = "J",
  JSON = "JSON",
  Java = "Java",
  JavaScript = "JavaScript",
  JavaScriptReact = "JavaScriptReact",
  Jsonnet = "Jsonnet",
  Julia = "Julia",
  Justfile = "Justfile",
  Kotlin = "Kotlin",
  LaTeX = "LaTeX",
  Lean = "Lean",
  Less = "Less",
  Lua = "Lua",
  Luau = "Luau",
  Makefile = "Makefile",
  Markdown = "Markdown",
  Matlab = "Matlab",
  Nickel = "Nickel",
  Nix = "Nix",
  OCaml = "OCaml",
  Objective_C = "Objective_C",
  Objective_CPP = "Objective_CPP",
  Pascal = "Pascal",
  PHP = "PHP",
  PLSQL = "PLSQL",
  Perl = "Perl",
  PowerShell = "PowerShell",
  Prolog = "Prolog",
  Protobuf = "Protobuf",
  Python = "Python",
  R = "R",
  Racket = "Racket",
  Raku = "Raku",
  Razor = "Razor",
  Repro = "Repro",
  ReST = "ReST",
  Ruby = "Ruby",
  Rust = "Rust",
  SAS = "SAS",
  SCSS = "SCSS",
  SML = "SML",
  SQL = "SQL",
  Sass = "Sass",
  Scala = "Scala",
  Scheme = "Scheme",
  ShellScript = "ShellScript",
  Skylark = "Skylark",
  Slang = "Slang",
  Solidity = "Solidity",
  Svelte = "Svelte",
  Swift = "Swift",
  Tcl = "Tcl",
  TOML = "TOML",
  TeX = "TeX",
  Thrift = "Thrift",
  TypeScript = "TypeScript",
  TypeScriptReact = "TypeScriptReact",
  Verilog = "Verilog",
  VHDL = "VHDL",
  VisualBasic = "VisualBasic",
  Vue = "Vue",
  Wolfram = "Wolfram",
  XML = "XML",
  XSL = "XSL",
  YAML = "YAML",
  Zig = "Zig",
}

export enum DescriptorSuffix {
  UnspecifiedSuffix = "UnspecifiedSuffix",
  Namespace = "Namespace",
  Package = "Package",
  Type = "Type",
  Term = "Term",
  Method = "Method",
  TypeParameter = "TypeParameter",
  Parameter = "Parameter",
  Meta = "Meta",
  Local = "Local",
  Macro = "Macro",
}

export enum SymbolInformationKind {
  UnspecifiedKind = "UnspecifiedKind",
  AbstractMethod = "AbstractMethod",
  Accessor = "Accessor",
  Array = "Array",
  Assertion = "Assertion",
  AssociatedType = "AssociatedType",
  Attribute = "Attribute",
  Axiom = "Axiom",
  Boolean = "Boolean",
  Class = "Class",
  Concept = "Concept",
  Constant = "Constant",
  Constructor = "Constructor",
  Contract = "Contract",
  DataFamily = "DataFamily",
  Delegate = "Delegate",
  Enum = "Enum",
  EnumMember = "EnumMember",
  Error = "Error",
  Event = "Event",
  Extension = "Extension",
  Fact = "Fact",
  Field = "Field",
  File = "File",
  Function = "Function",
  Getter = "Getter",
  Grammar = "Grammar",
  Instance = "Instance",
  Interface = "Interface",
  Key = "Key",
  Lang = "Lang",
  Lemma = "Lemma",
  Library = "Library",
  Macro = "Macro",
  Method = "Method",
  MethodAlias = "MethodAlias",
  MethodReceiver = "MethodReceiver",
  MethodSpecification = "MethodSpecification",
  Message = "Message",
  Mixin = "Mixin",
  Modifier = "Modifier",
  Module = "Module",
  Namespace = "Namespace",
  Null = "Null",
  Number = "Number",
  Object = "Object",
  Operator = "Operator",
  Package = "Package",
  PackageObject = "PackageObject",
  Parameter = "Parameter",
  ParameterLabel = "ParameterLabel",
  Pattern = "Pattern",
  Predicate = "Predicate",
  Property = "Property",
  Protocol = "Protocol",
  ProtocolMethod = "ProtocolMethod",
  PureVirtualMethod = "PureVirtualMethod",
  Quasiquoter = "Quasiquoter",
  SelfParameter = "SelfParameter",
  Setter = "Setter",
  Signature = "Signature",
  SingletonClass = "SingletonClass",
  SingletonMethod = "SingletonMethod",
  StaticDataMember = "StaticDataMember",
  StaticEvent = "StaticEvent",
  StaticField = "StaticField",
  StaticMethod = "StaticMethod",
  StaticProperty = "StaticProperty",
  StaticVariable = "StaticVariable",
  String = "String",
  Struct = "Struct",
  Subscript = "Subscript",
  Tactic = "Tactic",
  Theorem = "Theorem",
  ThisParameter = "ThisParameter",
  Trait = "Trait",
  TraitMethod = "TraitMethod",
  Type = "Type",
  TypeAlias = "TypeAlias",
  TypeClass = "TypeClass",
  TypeClassMethod = "TypeClassMethod",
  TypeFamily = "TypeFamily",
  TypeParameter = "TypeParameter",
  Union = "Union",
  Value = "Value",
  Variable = "Variable",
}

export type Index = {
  metadata?: Metadata;
  documents?: Document[];
  externalSymbols?: SymbolInformation[];
};

export type Metadata = {
  version?: ProtocolVersion;
  toolInfo?: ToolInfo;
  projectRoot?: string;
  textDocumentEncoding?: TextEncoding;
};

export type ToolInfo = {
  name?: string;
  version?: string;
  arguments?: string[];
};

export type Document = {
  language?: string;
  relativePath?: string;
  occurrences?: Occurrence[];
  symbols?: SymbolInformation[];
  text?: string;
  positionEncoding?: PositionEncoding;
};

export type Symbol = {
  scheme?: string;
  package?: Package;
  descriptors?: Descriptor[];
};

export type Package = {
  manager?: string;
  name?: string;
  version?: string;
};

export type Descriptor = {
  name?: string;
  disambiguator?: string;
  suffix?: DescriptorSuffix;
};

export type SymbolInformation = {
  symbol?: string;
  documentation?: string[];
  relationships?: Relationship[];
  kind?: SymbolInformationKind;
  displayName?: string;
  signatureDocumentation?: Document;
  enclosingSymbol?: string;
};

export type Relationship = {
  symbol?: string;
  isReference?: boolean;
  isImplementation?: boolean;
  isTypeDefinition?: boolean;
  isDefinition?: boolean;
};

export type Occurrence = {
  range?: number[];
  symbol?: string;
  symbolRoles?: number;
  overrideDocumentation?: string[];
  syntaxKind?: SyntaxKind;
  diagnostics?: Diagnostic[];
  enclosingRange?: number[];
};

export type Diagnostic = {
  severity?: Severity;
  code?: string;
  message?: string;
  source?: string;
  tags?: DiagnosticTag[];
};