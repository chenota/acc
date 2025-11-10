(in-package :acc)

(defclass token-sequence ()
    ((location :accessor location :initform 0)
     (token-list :accessor token-list :initarg :token-list))
  (:documentation "Token sequence manager used for convenience."))

(defmethod initialize-instance :after ((ts token-sequence) &key token-list &allow-other-keys)
  "Initialization logic for a token-sequence."
  (assert (typep token-list 'sequence))
  (assert (every #'token-p token-list))
  (setf (token-list ts) (coerce token-list 'vector)))

(defmethod make-token-sequence (token-list)
  "Create a token sequence"
  (make-instance 'token-sequence :token-list token-list))

(defmethod peek ((ts token-sequence))
  "Return the token at the curren position."
  (aref (token-list ts) (location ts)))

(defmethod advance ((ts token-sequence))
  "Advance the current position."
  (when (>= (location ts) (length (token-list ts))) (error "bad"))
  (prog1
      (peek ts)
    (incf (location ts))))