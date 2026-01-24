(asdf:defsystem :acc
  :description "Compiler for the acc language"
  :author "Alex Chenot"
  :license "MIT"
  :version "0.1.0"
  :serial t
  :build-operation "program-op"
  :build-pathname "acc"
  :entry-point "acc:main"
  :depends-on (:cl-ppcre :fiveam :unix-opts :uiop :alexandria)
  :components ((:file "package")
               (:module "src"
                        :components ((:module "shared"
                                              :components ((:file "util")
                                                           (:file "env")
                                                           (:file "ast")
                                                           (:file "error")))
                                     (:module "analysis"
                                              :components ((:file "static-type")))
                                     (:module "token"
                                              :components ((:file "lexer")
                                                           (:file "token-sequence")))
                                     (:module "parse"
                                              :components ((:file "expr")
                                                           (:file "program")
                                                           (:file "type")))
                                     (:module "codegen"
                                              :components ((:file "codegen")
                                                           (:file "instruction")))
                                     (:file "main")))
               (:module "test"
                        :components ((:file "lexer")
                                     (:file "instruction")
                                     (:file "token-sequence")
                                     (:file "expr")
                                     (:file "program")
                                     (:file "codegen")
                                     (:file "type")
                                     (:file "static-type")
                                     (:file "env")))))