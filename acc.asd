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
                        :components ((:file "util")
                                     (:file "error")
                                     (:file "ast")
                                     (:file "lexer")
                                     (:file "instruction")
                                     (:file "token-sequence")
                                     (:file "expr")
                                     (:file "program")
                                     (:file "codegen")
                                     (:file "main")
                                     (:file "type")))
               (:module "test"
                        :components ((:file "lexer")
                                     (:file "instruction")
                                     (:file "token-sequence")
                                     (:file "expr")
                                     (:file "program")
                                     (:file "codegen")
                                     (:file "type")))))