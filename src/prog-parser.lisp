(in-package :acc)

(defclass token-sequence ()
    ((location :accessor location :initform 0)
     (token-list :accessor token-list :initarg :token-list))
    (:documentation "Token sequence manager used for convenience."))

(defmethod initialize-instance :after ((ts token-sequence) &key token-list &allow-other-keys)
  "Initialization logic for a token-sequence."
  (assert (typep token-list 'sequence))
  (setf (token-list ts) (coerce token-list 'vector)))

(defmethod peek ((ts token-sequence))
  "Return the token at the curren position."
  (aref (token-list ts) (location ts)))

(defmethod advance ((ts token-sequence))
  "Advance the current position."
  (when (>= (location ts) (length (token-list ts))) (error "bad"))
  (prog1
      (peek ts)
    (incf (location ts))))

(defmethod expect ((ts token-sequence) kind)
  "Expect a single token."
  (assert (keywordp kind))
  (unless (eq (token-kind (peek ts)) kind) (error "bad"))
  (advance ts))

(defmethod expect-and ((ts token-sequence) &rest kinds)
  "Expect a sequence of tokens."
  (loop for kind in kinds
        collect (expect ts kind)))

(defun parse-program (token-list)
  (let ((ts (make-instance 'token-sequence :token-list token-list)))
    (expect ts :func)
    (let ((name (token-value (expect ts :ident)))
          (return-type (token-value (expect ts :ident))))
      (expect-and ts :lbrace :return)
      (let ((return-value (token-value (expect ts :int))))
        (expect-and ts :semi :rbrace)
        (assert (string= name "main"))
        (assert (string= return-type "int"))
        return-value))))