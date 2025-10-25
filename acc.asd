(asdf:defsystem :acc
  :description "Compiler for the acc language"
  :author "Alex Chenot"
  :license "MIT"
  :version "0.1.0"
  :serial t
  :depends-on (:cl-ppcre :fiveam)
  :components ((:file "package")
               (:module "test"
                        :components ((:file "lexer-test")))
               (:module "src"
                        :components ((:file "lexer")))))