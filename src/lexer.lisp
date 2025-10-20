(in-package :acc)

(defstruct token
  (kind nil :type keyword)
  value
  (row nil :type (integer 0 *))
  (col nil :type (integer 0 *)))

(defparameter tokens '(
  (:funckw "func")
  (:returnkw "return")
  (:semikw ";")
  (:lbrace "{")
  (:rbrace "}")
  (:ident "[a-z]+")
  (:int "[0-9]+")))