(in-package :acc)

(defstruct token
  (kind nil :type keyword)
  value
  (row nil :type (integer 0 *))
  (col nil :type (integer 0 *)))

(defparameter
  compiled-tokens
  (mapcar
      (lambda
          (token)
        (list
         (first token)
         (cl-ppcre:create-scanner
           (concatenate 'string "^" (second token)))))
      '((:funckw "func")
        (:returnkw "return")
        (:semikw ";")
        (:lbrace "{")
        (:rbrace "}")
        (:ident "[a-z]+")
        (:int "[0-9]+")
        (:white " "))))

(defun token-length (value)
  "Extract the length of a token."
  (check-type value (or sequence token))
  (if (typep value 'token)
      (length (token-value value))
      (length value)))

(defun tokenize (target)
  "Transform a string into a sequence of tokens."
  (check-type target string)
  (loop with i = 0 while (< i (length target))
        for best-match =
          (loop with match = nil
                for rule in compiled-tokens
                do
                  (multiple-value-bind
                      (new-match _)
                      (cl-ppcre:scan-to-strings (second rule) target :start i)
                    (declare (ignore _))
                    (when
                     (> (token-length new-match) (token-length match))
                     (setf match (make-token :kind (first rule) :value new-match :row i :col 0))
                     (incf i (length new-match))))
                finally (return match))
          unless best-match do (error "bad")
        collect best-match))