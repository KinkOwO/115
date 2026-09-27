// df_panel_caller_a = sub_1441D6570 0x1441D6570

__int64 __fastcall sub_1441D6570(_QWORD *a1, __int64 *a2, int a3)
{
  __int64 *v3; // r14
  _QWORD *v4; // rsi
  __int64 v5; // rax
  __int64 v6; // rax
  char v7; // di
  __int64 v8; // rdx
  __int64 v9; // rbx
  __int64 v10; // rax
  __int64 v11; // rdx
  __int64 v12; // rbx
  __int64 v13; // rax
  unsigned int v14; // eax
  char v15; // di
  __int64 v16; // rdx
  __int64 v17; // rbx
  __int64 v18; // rax
  __int64 v19; // rdx
  __int64 v20; // rdi
  __int64 v21; // rax
  unsigned int v22; // eax
  __int64 v23; // rdi
  _QWORD *v24; // rbx
  __int64 v25; // rax
  __int64 v26; // rax
  int v27; // eax
  char v28; // al
  int v29; // edx
  __int64 v30; // rcx
  __int64 v31; // rax
  void (__fastcall ***v32)(_QWORD); // rcx
  __int64 v33; // rcx
  __int64 v34; // rax
  void (__fastcall ***v35)(_QWORD); // rcx
  __int64 v36; // rdx
  __int64 v37; // rax
  __int64 v38; // rcx
  __int64 v39; // rax
  void (__fastcall ***v40)(_QWORD); // rcx
  __int64 v41; // rax
  void (__fastcall ***v42)(_QWORD); // rcx
  __int64 v43; // rbx
  __int64 v44; // rcx
  __int64 v45; // rax
  void (__fastcall ***v46)(_QWORD); // rcx
  int v47; // eax
  __int64 v48; // rdx
  __int64 v49; // rcx
  __int64 v50; // rax
  void (__fastcall ***v51)(_QWORD); // rcx
  __int64 v52; // rcx
  __int64 v53; // rax
  void (__fastcall ***v54)(_QWORD); // rcx
  __int64 v55; // rdx
  __int64 v56; // rax
  __int64 v57; // rcx
  __int64 v58; // rax
  void (__fastcall ***v59)(_QWORD); // rcx
  __int64 v60; // rax
  void (__fastcall ***v61)(_QWORD); // rcx
  __int64 v62; // rcx
  __int64 v63; // rax
  void (__fastcall ***v64)(_QWORD); // rcx
  int v65; // eax
  int v66; // ecx
  __int64 v67; // rcx
  __int64 v68; // rax
  void (__fastcall ***v69)(_QWORD); // rcx
  __int64 v70; // rcx
  __int64 v71; // rax
  void (__fastcall ***v72)(_QWORD); // rcx
  __int64 v73; // rax
  __int64 v74; // rcx
  __int64 v75; // rax
  void (__fastcall ***v76)(_QWORD); // rcx
  __int64 v77; // rcx
  __int64 v78; // rax
  void (__fastcall ***v79)(_QWORD); // rcx
  __int64 v80; // rcx
  __int64 v81; // rax
  void (__fastcall ***v82)(_QWORD); // rcx
  __int64 v83; // rax
  __int64 v84; // rcx
  __int64 v85; // rax
  void (__fastcall ***v86)(_QWORD); // rcx
  int v87; // eax
  int v88; // ecx
  __int64 v89; // rcx
  __int64 v90; // rax
  void (__fastcall ***v91)(_QWORD); // rcx
  __int64 v92; // rcx
  __int64 v93; // rax
  void (__fastcall ***v94)(_QWORD); // rcx
  __int64 v95; // rax
  __int64 v96; // rax
  __int64 v97; // rcx

  v3 = a2;
  v4 = a1;
  if ( a3 == 13 )
  {
    v5 = a1[9];
    if ( v5 && v5 == *a2 )
    {
      (*(void (__fastcall **)(_QWORD *))(*a1 + 112LL))(a1);
    }
    else
    {
      v6 = a1[11];
      if ( v6 && v6 == *a2 )
        (*(void (__fastcall **)(_QWORD *))(*a1 + 104LL))(a1);
    }
    if ( v4[19] == *v3 )
    {
      v7 = ((__int64 (*)(void))sub_146F03E70)();
      LOBYTE(v8) = v7 == 0;
      sub_1441D98B0(v4, v8);
      v9 = v4[19];
      v10 = sub_140764510();
      LOBYTE(v11) = (int)sub_1444EBF80(v10) <= 1;
      sub_146F01920(v9, v11);
      (*(void (__fastcall **)(_QWORD *, _QWORD))(*v4 + 32LL))(v4, 0);
      if ( !v7 )
      {
        v12 = v4[1];
        v13 = sub_140764510();
        v14 = sub_1444EBF60(v13);
        sub_1441E0820(v12, 1, v14);
      }
    }
    if ( v4[184] == *v3 )
    {
      v15 = ((__int64 (*)(void))sub_146F03E70)();
      LOBYTE(v16) = v15 == 0;
      sub_1441D9B40(v4, v16);
      v17 = v4[184];
      v18 = sub_140764510();
      LOBYTE(v19) = (int)sub_1444EBFC0(v18) <= 1;
      sub_146F01920(v17, v19);
      (*(void (__fastcall **)(_QWORD *, _QWORD))(*v4 + 32LL))(v4, 0);
      if ( !v15 )
      {
        v20 = v4[1];
        v21 = sub_140764510();
        v22 = sub_1444EBFA0(v21);
        sub_1441E0820(v20, 1, v22);
      }
    }
    v23 = 0;
    while ( 1 )
    {
      sub_1421B2800(v4[14]);
      v24 = &v4[52 * v23];
      v25 = sub_1444EBAB0(*((unsigned int *)v24 + 52));
      if ( !v25 || *(_DWORD *)(v25 + 8) != 1 )
        goto LABEL_162;
      v26 = *v3;
      if ( *v3 == v24[27] )
      {
        sub_1441E0820(v4[1], 1, *((unsigned int *)v24 + 52));
        (*(void (__fastcall **)(_QWORD *, _QWORD))(*v4 + 32LL))(v4, 0);
        v27 = sub_146E8C7D0(&unk_149242AE8);
        sub_145A31380(v27, -1, 0, 0, -1, -1, 0);
      }
      else
      {
        if ( v26 == v24[47] )
        {
          v28 = sub_146F03E70(v24[45]);
          v29 = *((_DWORD *)v4 + 362);
          if ( !v29 )
          {
            v49 = qword_14E638F28;
            if ( v28 )
            {
              if ( !qword_14E638F28 )
              {
                v60 = sub_146E8BA20(1472);
                if ( v60 )
                  v61 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v60);
                else
                  v61 = 0;
                qword_14E638F28 = (__int64)v61;
                (**v61)(v61);
                v49 = qword_14E638F28;
              }
              sub_1444E9A40(v49, *((unsigned int *)v24 + 52));
            }
            else
            {
              if ( !qword_14E638F28 )
              {
                v50 = sub_146E8BA20(1472);
                if ( v50 )
                  v51 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v50);
                else
                  v51 = 0;
                qword_14E638F28 = (__int64)v51;
                (**v51)(v51);
                v49 = qword_14E638F28;
              }
              if ( !(unsigned __int8)sub_1444EC6E0(v49, *((unsigned int *)v24 + 52)) )
              {
                v52 = qword_14E638F28;
                if ( !qword_14E638F28 )
                {
                  v53 = sub_146E8BA20(1472);
                  if ( v53 )
                    v54 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v53);
                  else
                    v54 = 0;
                  qword_14E638F28 = (__int64)v54;
                  (**v54)(v54);
                  v52 = qword_14E638F28;
                }
                if ( (int)sub_1444EBF80(v52) < 10 )
                {
                  v57 = qword_14E638F28;
                  if ( !qword_14E638F28 )
                  {
                    v58 = sub_146E8BA20(1472);
                    if ( v58 )
                      v59 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v58);
                    else
                      v59 = 0;
                    qword_14E638F28 = (__int64)v59;
                    (**v59)(v59);
                    v57 = qword_14E638F28;
                  }
                  sub_1444E9240(v57, *((unsigned int *)v24 + 52));
                  sub_1441E0820(v4[1], 1, *((unsigned int *)v24 + 52));
                }
                else
                {
                  if ( qword_14E683C78 )
                  {
                    v56 = sub_14723C170(100002259);
                    sub_14668C520(qword_14E683C78, 2875, v56, 0);
                  }
                  LOBYTE(v55) = 1;
                  sub_146F01920(v24[45], v55);
                }
              }
            }
            v43 = v4[19];
            v62 = qword_14E638F28;
            if ( !qword_14E638F28 )
            {
              v63 = sub_146E8BA20(1472);
              if ( v63 )
                goto LABEL_135;
              v64 = 0;
              goto LABEL_158;
            }
            goto LABEL_159;
          }
          if ( v29 != 1 )
            goto LABEL_161;
          v30 = qword_14E638F28;
          if ( v28 )
          {
            if ( !qword_14E638F28 )
            {
              v41 = sub_146E8BA20(1472);
              if ( v41 )
                v42 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v41);
              else
                v42 = 0;
              qword_14E638F28 = (__int64)v42;
              (**v42)(v42);
              v30 = qword_14E638F28;
            }
            sub_1444E9AD0(v30, *((unsigned int *)v24 + 52));
          }
          else
          {
            if ( !qword_14E638F28 )
            {
              v31 = sub_146E8BA20(1472);
              if ( v31 )
                v32 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v31);
              else
                v32 = 0;
              qword_14E638F28 = (__int64)v32;
              (**v32)(v32);
              v30 = qword_14E638F28;
            }
            if ( !(unsigned __int8)sub_1444EC710(v30, *((unsigned int *)v24 + 52)) )
            {
              v33 = qword_14E638F28;
              if ( !qword_14E638F28 )
              {
                v34 = sub_146E8BA20(1472);
                if ( v34 )
                  v35 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v34);
                else
                  v35 = 0;
                qword_14E638F28 = (__int64)v35;
                (**v35)(v35);
                v33 = qword_14E638F28;
              }
              if ( (int)sub_1444EBFC0(v33) < 10 )
              {
                v38 = qword_14E638F28;
                if ( !qword_14E638F28 )
                {
                  v39 = sub_146E8BA20(1472);
                  if ( v39 )
                    v40 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v39);
                  else
                    v40 = 0;
                  qword_14E638F28 = (__int64)v40;
                  (**v40)(v40);
                  v38 = qword_14E638F28;
                }
                sub_1444E9290(v38, *((unsigned int *)v24 + 52));
                sub_1441E0820(v4[1], 1, *((unsigned int *)v24 + 52));
              }
              else
              {
                if ( qword_14E683C78 )
                {
                  v37 = sub_14723C170(100002259);
                  sub_14668C520(qword_14E683C78, 2875, v37, 0);
                }
                LOBYTE(v36) = 1;
                sub_146F01920(v24[45], v36);
              }
            }
          }
          v43 = v4[184];
          v44 = qword_14E638F28;
          if ( qword_14E638F28 )
            goto LABEL_55;
          v45 = sub_146E8BA20(1472);
          if ( !v45 )
          {
            v46 = 0;
            goto LABEL_54;
          }
LABEL_147:
          v46 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v45);
          goto LABEL_54;
        }
        if ( v26 == v24[49] )
        {
          v65 = sub_146E8C7D0(&unk_149242AE8);
          sub_145A31380(v65, -1, 0, 0, -1, -1, 0);
          v66 = *((_DWORD *)v4 + 362);
          if ( !v66 )
          {
            v77 = qword_14E638F28;
            if ( !qword_14E638F28 )
            {
              v78 = sub_146E8BA20(1472);
              if ( v78 )
                v79 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v78);
              else
                v79 = 0;
              qword_14E638F28 = (__int64)v79;
              (**v79)(v79);
              v77 = qword_14E638F28;
            }
            if ( !(unsigned __int8)sub_1444EC6E0(v77, *((unsigned int *)v24 + 52)) )
            {
              v80 = qword_14E638F28;
              if ( !qword_14E638F28 )
              {
                v81 = sub_146E8BA20(1472);
                if ( v81 )
                  v82 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v81);
                else
                  v82 = 0;
                qword_14E638F28 = (__int64)v82;
                (**v82)(v82);
                v80 = qword_14E638F28;
              }
              if ( (int)sub_1444EBF80(v80) < 10 )
              {
                v84 = qword_14E638F28;
                if ( !qword_14E638F28 )
                {
                  v85 = sub_146E8BA20(1472);
                  if ( v85 )
                    v86 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v85);
                  else
                    v86 = 0;
                  qword_14E638F28 = (__int64)v86;
                  (**v86)(v86);
                  v84 = qword_14E638F28;
                }
                sub_1444E9240(v84, *((unsigned int *)v24 + 52));
                sub_1441E0820(v4[1], 1, *((unsigned int *)v24 + 52));
              }
              else if ( qword_14E683C78 )
              {
                v83 = sub_14723C170(100002259);
                sub_14668C520(qword_14E683C78, 2875, v83, 0);
              }
            }
            v43 = v4[19];
            v62 = qword_14E638F28;
            if ( !qword_14E638F28 )
            {
              v63 = sub_146E8BA20(1472);
              if ( v63 )
                goto LABEL_135;
              v64 = 0;
              goto LABEL_158;
            }
            goto LABEL_159;
          }
          if ( v66 != 1 )
            goto LABEL_161;
          v67 = qword_14E638F28;
          if ( !qword_14E638F28 )
          {
            v68 = sub_146E8BA20(1472);
            if ( v68 )
              v69 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v68);
            else
              v69 = 0;
            qword_14E638F28 = (__int64)v69;
            (**v69)(v69);
            v67 = qword_14E638F28;
          }
          if ( !(unsigned __int8)sub_1444EC710(v67, *((unsigned int *)v24 + 52)) )
          {
            v70 = qword_14E638F28;
            if ( !qword_14E638F28 )
            {
              v71 = sub_146E8BA20(1472);
              if ( v71 )
                v72 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v71);
              else
                v72 = 0;
              qword_14E638F28 = (__int64)v72;
              (**v72)(v72);
              v70 = qword_14E638F28;
            }
            if ( (int)sub_1444EBFC0(v70) < 10 )
            {
              v74 = qword_14E638F28;
              if ( !qword_14E638F28 )
              {
                v75 = sub_146E8BA20(1472);
                if ( v75 )
                  v76 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v75);
                else
                  v76 = 0;
                qword_14E638F28 = (__int64)v76;
                (**v76)(v76);
                v74 = qword_14E638F28;
              }
              sub_1444E9290(v74, *((unsigned int *)v24 + 52));
              sub_1441E0820(v4[1], 1, *((unsigned int *)v24 + 52));
            }
            else if ( qword_14E683C78 )
            {
              v73 = sub_14723C170(100002259);
              sub_14668C520(qword_14E683C78, 2875, v73, 0);
            }
          }
          v43 = v4[184];
          v44 = qword_14E638F28;
          if ( qword_14E638F28 )
            goto LABEL_55;
          v45 = sub_146E8BA20(1472);
          if ( !v45 )
          {
            v46 = 0;
            goto LABEL_54;
          }
          goto LABEL_147;
        }
        if ( v26 == v24[51] )
        {
          v87 = sub_146E8C7D0(&unk_149242AE8);
          sub_145A31380(v87, -1, 0, 0, -1, -1, 0);
          v88 = *((_DWORD *)v4 + 362);
          if ( !v88 )
          {
            v92 = qword_14E638F28;
            if ( !qword_14E638F28 )
            {
              v93 = sub_146E8BA20(1472);
              if ( v93 )
                v94 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v93);
              else
                v94 = 0;
              qword_14E638F28 = (__int64)v94;
              (**v94)(v94);
              v92 = qword_14E638F28;
            }
            sub_1444E9A40(v92, *((unsigned int *)v24 + 52));
            v43 = v4[19];
            v62 = qword_14E638F28;
            if ( !qword_14E638F28 )
            {
              v63 = sub_146E8BA20(1472);
              if ( v63 )
LABEL_135:
                v64 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v63);
              else
                v64 = 0;
LABEL_158:
              qword_14E638F28 = (__int64)v64;
              (**v64)(v64);
              v62 = qword_14E638F28;
            }
LABEL_159:
            v47 = sub_1444EBF80(v62);
            goto LABEL_160;
          }
          if ( v88 == 1 )
          {
            v89 = qword_14E638F28;
            if ( !qword_14E638F28 )
            {
              v90 = sub_146E8BA20(1472);
              if ( v90 )
                v91 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v90);
              else
                v91 = 0;
              qword_14E638F28 = (__int64)v91;
              (**v91)(v91);
              v89 = qword_14E638F28;
            }
            sub_1444E9AD0(v89, *((unsigned int *)v24 + 52));
            v43 = v4[184];
            v44 = qword_14E638F28;
            if ( !qword_14E638F28 )
            {
              v45 = sub_146E8BA20(1472);
              if ( v45 )
                goto LABEL_147;
              v46 = 0;
LABEL_54:
              qword_14E638F28 = (__int64)v46;
              (**v46)(v46);
              v44 = qword_14E638F28;
            }
LABEL_55:
            v47 = sub_1444EBFC0(v44);
LABEL_160:
            LOBYTE(v48) = v47 > 1;
            sub_146F01920(v43, v48);
LABEL_161:
            (*(void (__fastcall **)(_QWORD *, _QWORD))(*v4 + 32LL))(v4, 0);
          }
        }
      }
LABEL_162:
      if ( ++v23 >= 3 )
        return 0;
    }
  }
  if ( a3 == 24 )
  {
    v95 = *a2;
    if ( *a2 == a1[14] )
    {
      v96 = *a1;
      a2 = 0;
LABEL_173:
      (*(void (__fastcall **)(_QWORD *, __int64 *))(v96 + 32))(a1, a2);
      return 0;
    }
    if ( v95 == a1[5] )
    {
      v96 = *a1;
LABEL_172:
      LOBYTE(a2) = 1;
      goto LABEL_173;
    }
    v97 = a1[182];
    if ( v97 && v95 == v97 )
    {
      (*(void (__fastcall **)(_QWORD *))(*v4 + 104LL))(v4);
      *((_DWORD *)v4 + 32) = -1;
      *((_DWORD *)v4 + 362) = sub_146F50350(v4[182]);
      sub_1441ECDB0(v4[1], 1);
      (*(void (__fastcall **)(_QWORD *))(*v4 + 24LL))(v4);
      v96 = *v4;
      a1 = v4;
      goto LABEL_172;
    }
  }
  return 0;
}

