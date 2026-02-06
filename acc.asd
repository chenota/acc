(asdf:defsystem :acc
  :description "Compiler for the acc language"
  :author "Alex Chenot"
  :license "MIT"
  :version "0.1.0"
  :serial t
  :build-operation "program-op"
  :build-pathname "acc"
  :entry-point "acc:main"
  :depends-on (:cl-ppcre :unix-opts :uiop :alexandria)
  :components ((:file "package/main")
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
                                     (:file "main")))))
(asdf:defsystem :acc/test
  :description "Tests for the acc language"
  :author "Alex Chenot"
  :license "MIT"
  :serial t
  :depends-on (:fiveam :acc)
  :components ((:file "package/test")
               (:module "test"
                        :components ((:file "helper")
                                     (:file "lexer")
                                     (:file "instruction")
                                     (:file "token-sequence")
                                     (:file "expr")
                                     (:file "program")
                                     (:file "codegen")
                                     (:file "type")
                                     (:file "static-type")
                                     (:file "env")))))