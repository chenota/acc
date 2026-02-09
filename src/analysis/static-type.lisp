(in-package :acc)

(with-ignore-coverage
  (defstruct assign-type-environment
    (function-return-context nil))
  (defgeneric assign-type (node env)
    (:documentation "Assigns and returns the type of the node within the given environment."))
  (defgeneric in-bounds-p (node size)
    (:documentation "Checks if a node is in bounds with respect to type."))
  (defmacro with-location-error (node &body forms)
    `(handler-case
         (progn ,@forms)
       (t (c) (error 'location-error :location (ast-node-location ,node) :message (format nil "~A" c))))))

(defun set-program-types (ast)
  "Perform type analysis on ast."
  (assert (program-node-p ast))
  (assign-type ast (make-env))
  ast)

(defmethod assign-type ((node program-node) env)
  (loop for fun in (program-node-functions node) do (assign-type fun env))
  node)

(defmethod assign-type ((node function-node) env)
  (let ((inner-env (env-extend env)))
    (setf (env-return-type inner-env) (function-node-return-type node))
    (loop for stmt in (function-node-body node) do (assign-type stmt inner-env))
    (setf
      (function-node-type-info node)
      (make-function-type :parameters nil :return-type (function-node-return-type node)))
    node))

(defmethod assign-type ((node return-statement-node) env)
  (let* ((e1 (assign-type (return-statement-node-expression node) env))
         (return-type (with-location-error node (find-return-type env)))
         (e1-cast (make-cast e1 return-type (ast-node-location node))))
    (setf (return-statement-node-expression node) e1-cast)
    node))

(defmethod assign-type ((node cast-node) env)
  (let ((e1 (assign-type (cast-node-expression node) env)))
    (make-cast e1 (cast-node-cast-type node) (ast-node-location node))))

(defmethod assign-type ((node declaration-node) env)
  (let* ((e1 (assign-type (declaration-node-expression node) env))
         (e1-cast (make-cast e1 (declaration-node-var-type node) (ast-node-location node))))
    (setf (declaration-node-expression node) e1-cast)
    (with-location-error
        (ast-node-location node)
      (register-symbol env (declaration-node-name node) (declaration-node-var-type node)))
    node))

(defmethod assign-type ((node assignment-node) env)
  (let* ((e1 (assign-type (assignment-node-expression node) env))
         (sym (with-location-error node (find-env-symbol env (assignment-node-name node))))
         (e1-cast (make-cast e1 (env-symbol-sym-type sym) (ast-node-location node))))
    (setf (assignment-node-expression node) e1-cast)
    node))

(defmethod assign-type ((node int-node) env)
  (setf (int-node-type-info node) (make-integer-type :size :generic))
  node)

(defmethod assign-type ((node ident-node) env)
  (setf
    (ast-node-type-info node)
    (env-symbol-sym-type (with-location-error node (find-env-symbol env (ident-node-name node)))))
  node)

(defun make-cast (source-node destination-type location)
  "Generate logic and AST modifications to cast SOURCE-NODE as DESTINATION-TYPE"
  (unless (valid-cast-p source-node destination-type)
    (error
        'location-error
      :location location
      :message (format
                   nil
                   "Invalid type cast: ~A as ~A"
                 (ast-node-type-info source-node)
                 destination-type)))
  (cond
   ;; Fold direct integer casts
   ((int-node-p source-node)
     (setf (ast-node-type-info source-node) destination-type)
     source-node)
   ;; Create an explicit cast for later codegen
   (t (make-cast-node :location location :expression source-node))))

(defun valid-cast-p (source-node destination-type)
  "Check if source can be type cast as destination."
  (cond
   ((int-node-p source-node) (in-bounds-p source-node destination-type))
   ((and
     (integer-type-p (ast-node-type-info source-node))
     (integer-type-p destination-type))
     t)
   (t (error 'location-error :location (ast-node-location source-node) :message "Unknown type cast"))))

(defmethod in-bounds-p ((node int-node) size)
  (assert (integer-type-p size))
  (let ((v (int-node-value node))
        (s (integer-type-size size)))
    (alexandria:switch (s)
      (:int8 (<= (s-min 8) v (s-max 8)))
      (:int16 (<= (s-min 16) v (s-max 16)))
      (:int32 (<= (s-min 32) v (s-max 32)))
      (:int64 (<= (s-min 64) v (s-max 64)))
      (:generic t)
      (t nil))))

(defun s-max (n)
  "Maximum value of a singed integer with n bits."
  (1- (ash 1 (1- n))))

(defun s-min (n)
  "Minimum value of a signed integer with n bits."
  (- (ash 1 (1- n))))