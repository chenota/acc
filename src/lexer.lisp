(in-package :acc)

(defstruct token
  (kind nil :type keyword)
  value
  (row nil :type (integer 0 *))
  (col nil :type (integer 0 *)))