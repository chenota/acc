(asdf:defsystem :acc
  :description "Compiler for the acc language"
  :author "Alex Chenot"
  :license "MIT"
  :version "0.1.0"
  :serial t
  :depends-on (:cl-ppcre :fiveam)
  :components ((:file "package")
               (:module "src"
                        :components ((:file "util")
                                     (:file "lexer")
                                     (:file "instruction")
                                     (:file "token-sequence")
                                     (:file "expr")
                                     (:file "program")
                                     (:file "codegen")))
               (:module "test"
                        :components ((:file "lexer-test")
                                     (:file "instruction-test")
                                     (:file "token-sequence-test")
                                     (:file "expr-test")
                                     (:file "program-test")
                                     (:file "codegen-test")))))