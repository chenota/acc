(in-package :acc)

(with-ignore-coverage
 (defstruct token
   (kind nil :type keyword)
   (value nil :type t)
   (loc nil :type t)
   (len nil :type (integer 0 *)))

 (defparameter
   +compiled-tokens+
   (mapcar
       (lambda
           (token)
         (list
          (first token)
          (cl-ppcre:create-scanner
           (concatenate 'string "^" (second token)))
          (third token)))
       `((:fun "fun" t)
         (:return "return" t)
         (:semi ";" t)
         (:lbrace "\\{" t)
         (:rbrace "\\}" t)
         (:lparen "\\(" t)
         (:rparen "\\)" t)
         (:let "let" t)
         (:colon ":" t)
         (:equal "=" t)
         (:ident "[a-zA-Z][a-zA-Z0-9]*" identity)
         (:int "[0-9]+" parse-integer)
         (:whitespace " " nil)
         (:newline "\\n" nil)))))

(defun tokenize (target)
  "Transform a string into a sequence of tokens."
  (check-type target string)
  (loop with row = 0
        with col = 0
        with i = 0
        while (< i (length target))
        for best-match =
          (loop with match = nil
                with matched-rule = nil
                for rule in +compiled-tokens+
                do
                  (multiple-value-bind
                      (new-match _)
                      (cl-ppcre:scan-to-strings (second rule) target :start i)
                    (declare (ignore _))
                    (when
                     (> (length new-match) (length match))
                     (setf match new-match)
                     (setf matched-rule rule)))
                finally
                  (progn
                   (incf i (length match))
                   (return
                     (if match
                         (prog1
                             (if
                              (third matched-rule)
                              (make-token
                               :kind (first matched-rule)
                               :value (handler-case
                                          (funcall (third matched-rule) match)
                                        (undefined-function (e) (declare (ignore e)) match))
                               :loc (list row col)
                               :len (length match)))
                           (if
                            (eq (first matched-rule) :newline)
                            (progn (setf col 0) (incf row))
                            (incf col (length match))))
                         (error "bad")))))
          when best-match collect best-match))
