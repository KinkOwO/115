// singleton_user_sub_1444A30D0

__int64 __fastcall sub_1444A30D0(__int64 a1, __int64 *a2)
{
  __int64 *v2; // rbx
  int v4; // esi
  int v5; // r12d
  __int64 v6; // r14
  __int64 result; // rax
  __int64 v8; // rcx
  __int64 v9; // rax
  void (__fastcall ***v10)(_QWORD); // rcx
  __int64 v11; // rdi
  __int64 v12; // r13
  __int64 *v13; // rcx
  __int64 v14; // rdx
  __int64 v15; // rcx
  int v16; // eax
  __int64 v17; // rcx
  __int64 v18; // rax
  void (__fastcall ***v19)(_QWORD); // rcx
  __int64 v20; // rcx
  __int64 v21; // rax
  void (__fastcall ***v22)(_QWORD); // rcx
  unsigned int v23; // ebp
  void (__fastcall *v24)(__int64, __int64, __int64, _QWORD, int, int, int, _DWORD, _DWORD, int, int); // rsi
  double v25; // xmm0_8
  int v26; // edi
  unsigned int v27; // ebx
  __int64 v28; // rdx
  __int64 v29; // r8
  __int64 v30; // rdi
  void (__fastcall *v31)(__int64, __int64); // rbx
  __int64 v32; // rax
  __int64 v33; // rdi
  void (__fastcall *v34)(__int64, __int64); // rbx
  __int64 v35; // rax
  __int64 v36; // rdi
  void (__fastcall *v37)(__int64, __int64); // rbx
  __int64 v38; // rax
  __int64 v39; // rax
  __int64 v40; // rax
  __int64 v41; // rdi
  void (__fastcall *v42)(__int64, __int64); // rbx
  __int64 v43; // rax
  __int64 v44; // rax
  __int64 v45; // rax
  int v46; // [rsp+28h] [rbp-E0h]
  int v47; // [rsp+50h] [rbp-B8h]
  _BYTE v48[16]; // [rsp+80h] [rbp-88h] BYREF
  _BYTE v49[16]; // [rsp+90h] [rbp-78h] BYREF
  unsigned int v50; // [rsp+110h] [rbp+8h]
  unsigned int v52; // [rsp+120h] [rbp+18h]
  int v53; // [rsp+128h] [rbp+20h]

  v2 = a2;
  sub_1467A6790(a1, a2);
  v4 = sub_145F12D90(qword_14E683C20);
  v53 = v4;
  v5 = 0;
  v6 = 0;
  do
  {
    result = *v2;
    if ( *(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 16) == *v2 )
    {
      v8 = qword_14E659EA8;
      if ( !qword_14E659EA8 )
      {
        v9 = sub_146E8BA20(496);
        if ( v9 )
          v10 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v9);
        else
          v10 = 0;
        qword_14E659EA8 = (__int64)v10;
        (**v10)(v10);
        v8 = qword_14E659EA8;
      }
      v11 = (unsigned int)sub_14449E110(v8, (unsigned int)v5);
      v12 = sub_145F13700(qword_14E683C20, v11);
      v13 = *(__int64 **)(v6 + *(_QWORD *)(a1 + 1488) + 32);
      v14 = *v13;
      LOBYTE(v14) = v12 != 0;
      (*(void (__fastcall **)(__int64 *, __int64))(*v13 + 16))(v13, v14);
      sub_146EF7900(*(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 48), v12 != 0);
      sub_146EF7900(*(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 80), v12 != 0);
      result = sub_146EF7900(*(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 64), v12 != 0);
      if ( (unsigned int)v5 < 3 )
      {
        v15 = *(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 96);
        result = (*(__int64 (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v15 + 16LL))(v15, 0);
      }
      if ( v12 )
      {
        v16 = dword_14F1C0880;
        if ( v4 == (_DWORD)v11 )
          v16 = dword_14F1C0A0C;
        v52 = v16;
        v17 = qword_14E659EA8;
        if ( !qword_14E659EA8 )
        {
          v18 = sub_146E8BA20(496);
          if ( v18 )
            v19 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v18);
          else
            v19 = 0;
          qword_14E659EA8 = (__int64)v19;
          (**v19)(v19);
          v17 = qword_14E659EA8;
        }
        v50 = sub_14449E100(v17, (unsigned int)v11);
        v20 = qword_14E659EA8;
        if ( !qword_14E659EA8 )
        {
          v21 = sub_146E8BA20(496);
          if ( v21 )
            v22 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v21);
          else
            v22 = 0;
          qword_14E659EA8 = (__int64)v22;
          (**v22)(v22);
          v20 = qword_14E659EA8;
        }
        v23 = sub_14449E0F0(v20, (unsigned int)v11);
        v24 = *(void (__fastcall **)(__int64, __int64, __int64, _QWORD, int, int, int, _DWORD, _DWORD, int, int))(*(_QWORD *)v12 + 16LL);
        v25 = sub_146EC9470(*a2);
        v26 = (int)*(float *)&v25;
        v27 = (int)sub_146ECA0B0(*a2);
        sub_142757420(*a2);
        sub_146ECA0C0(*a2);
        LOBYTE(v47) = 0;
        LOBYTE(v46) = 0;
        v24(v12, v28, v29, v27, v26, v46, 255, 0, 0, 1065353216, v47);
        v30 = *(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 32);
        v31 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v30 + 688LL);
        v32 = sub_141D37640(v12);
        v31(v30, v32);
        v33 = *(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 48);
        v34 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v33 + 688LL);
        v35 = sub_145F003E0(v12);
        v34(v33, v35);
        v36 = *(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 80);
        v37 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v36 + 688LL);
        v38 = sub_14723C170(100001118);
        v39 = sub_146E8CF20(v48, v38, v50);
        v40 = sub_14014F430(v39);
        v37(v36, v40);
        sub_146E8C910(v48);
        v41 = *(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 64);
        v42 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v41 + 688LL);
        v43 = sub_14723C170(100001119);
        v44 = sub_146E8CF20(v49, v43, v23);
        v45 = sub_14014F430(v44);
        v42(v41, v45);
        sub_146E8C910(v49);
        sub_146EF7900(*(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 32), v52);
        sub_146EF7900(*(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 48), v52);
        sub_146EF7900(*(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 80), v52);
        result = sub_146EF7900(*(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 64), v52);
        v4 = v53;
      }
      v2 = a2;
    }
    ++v5;
    v6 += 112;
  }
  while ( v5 < 8 );
  return result;
}

