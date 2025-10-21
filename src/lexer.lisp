(in-package :acc)

(defstruct token
  (kind nil :type keyword)
  (value nil :type t)
  (row nil :type (integer 0 *))
  (col nil :type (integer 0 *))
  (len nil :type (integer 0 *)))

(defparameter
  compiled-tokens
  (mapcar
      (lambda
          (token)
        (list
         (first token)
         (cl-ppcre:create-scanner
           (concatenate 'string "^" (second token)))
         (third token)))
      `((:funckw "func" t)
        (:returnkw "return" t)
        (:semikw ";" t)
        (:lbrace "\{" t)
        (:rbrace "\}" t)
        (:ident "[a-z]+" identity)
        (:int "[0-9]+" parse-integer)
        (:whitespace " " nil)
        (:newline "\n" nil))))

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
                for rule in compiled-tokens
                do
                  (multiple-value-bind
                      (new-match _)
                      (cl-ppcre:scan-to-strings (second rule) target :start i)
                    (declare (ignore _))
                    (when
                     (> (length new-match) (length match))
                     (setf match new-match)
                     (setf matched-rule rule)))
                finally (progn
                         (incf i (length match))
                         (return (if match
                                     (prog1
                                         (make-token
                                           :kind (first matched-rule)
                                           :value (cond
                                                   ((functionp (third matched-rule)) (funcall (third matched-rule) match))
                                                   ((third matched-rule) match))
                                           :row row
                                           :col col
                                           :len (length match))
                                       (if
                                        (eq (first matched-rule) :newline)
                                        (progn (setf row 0) (incf col))
                                        (incf row (length match))))
                                     (error "bad")))))
        collect best-match))